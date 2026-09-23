package controller

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	http2 "net/http"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/creack/pty"
	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	applicationLogic "github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/app/common/logic"
	statLogic "github.com/donknap/dpanel/app/common/logic/stat"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/stats"
	"github.com/donknap/dpanel/common/service/exec/local"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/plugin"
	"github.com/donknap/dpanel/common/service/ssh"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/patrickmn/go-cache"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	ssh2 "golang.org/x/crypto/ssh"
	"gorm.io/datatypes"
	"gorm.io/gen"
	"gorm.io/gorm"
)

type command struct {
	Type string `json:"type"`
	Size struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"size"`
	Content struct {
		Command string `json:"command"`
	} `json:"content"`
}

type Home struct {
	controller.Abstract
}

func (self Home) Index(http *gin.Context) {
	uri := http.Request.URL.String()
	slog.Debug("http route not found", "ip", http.ClientIP(), "user-agent", http.Request.Header.Get("User-Agent"), "uri", uri)
	var asset embed.FS
	if v, ok := storage.Cache.Get(storage.CacheKeyAsset); ok {
		asset = v.(embed.FS)
	} else {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageUnknow, "error", define.ErrorAssetEmpty.Error()), 500)
		return
	}
	// 如果没发现语言包返回默认的英文，并提示用户
	if strings.HasPrefix(uri, "/dpanel/static/asset/i18n") {
		enUs, _ := asset.ReadFile("asset/static/i18n/en-US.json")
		http.Data(http2.StatusOK, "application/json; charset=UTF-8", enUs)
		return
	}

	indexHtml, _ := asset.ReadFile("asset/static/index.html")
	for o, n := range map[string]string{
		"/favicon.ico": function.RouterUri("/favicon.ico"),
		"/dpanel":      function.RouterUri("/dpanel"),
	} {
		indexHtml = bytes.ReplaceAll(indexHtml, []byte(o), []byte(n))
	}
	http.Data(http2.StatusOK, "text/html; charset=UTF-8", indexHtml)
	return
}

func (self Home) WsNotice(http *gin.Context) {
	if !websocket.IsWebSocketUpgrade(http.Request) {
		self.JsonResponseWithError(http, errors.New("please connect using websocket"), 500)
		return
	}

	request := http.Copy()
	var explorerOpen atomic.Bool
	destroyExplorer := func() {
		dockerSdk, err := docker.NewClientWithUser(request)
		if err != nil {
			slog.Warn("get Docker client for explorer cleanup", "error", err)
			return
		}
		go func() {
			if err := (logic.Explorer{}).DestroyProxyContainer(dockerSdk); err != nil {
				slog.Warn("destroy explorer from notice websocket", "dockerEnv", dockerSdk.Name, "error", err)
			}
		}()
	}
	// Explorer 复用 notice 连接；断线时依据连接上的打开标记清理，不再维持独立 WebSocket。
	client, err := ws.NewClient(http,
		ws.WithMessageRecvHandler(ws.MessageTypeContainerExplorerOpen, func(*ws.RecvMessage) {
			explorerOpen.Store(true)
		}),
		ws.WithMessageRecvHandler(ws.MessageTypeContainerExplorerCancel, func(*ws.RecvMessage) {
			explorerOpen.Store(false)
		}),
		ws.WithMessageRecvHandler(ws.MessageTypeContainerExplorerDestroy, func(*ws.RecvMessage) {
			explorerOpen.Store(false)
			destroyExplorer()
		}),
		ws.WithCloseHandler(func() {
			if explorerOpen.Swap(false) {
				destroyExplorer()
			}
		}),
	)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	go client.ReadMessage()

	// 将自己的fd推回给客户端
	err = client.SendMessage(&ws.RespMessage{
		Type: ws.MessageTypeEventFd,
		Data: client.Fd,
	})
	if err != nil {
		slog.Warn("websocket", "connect", err.Error())
	}
}

func (self Home) WsContainerConsole(http *gin.Context) {
	if !websocket.IsWebSocketUpgrade(http.Request) {
		self.JsonResponseWithError(http, errors.New("please connect using websocket"), 500)
		return
	}
	type ParamsValidate struct {
		Id      string `json:"id" uri:"id" binding:"required"`
		Width   uint   `json:"width" form:"width"`
		Height  uint   `json:"height" form:"height"`
		Cmd     string `json:"cmd" form:"cmd"`
		User    string `json:"user" form:"user"`
		WorkDir string `json:"workDir" form:"workDir"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	params.Cmd = strings.TrimSpace(params.Cmd)
	if params.Cmd == "" {
		params.Cmd = "/bin/sh"
	}
	if params.Cmd == "bin/sh" || params.Cmd == "bin/bash" || params.Cmd == "bin/ash" {
		params.Cmd = "/" + params.Cmd
	}
	containerName := params.Id
	if _, pluginName, exists := strings.Cut(params.Id, ":"); exists {
		containerName = pluginName
	}
	if params.WorkDir == "" {
		params.WorkDir = "/"
	}
	var err error
	var shell types.HijackedResponse
	var execID string
	var client *ws.Client

	messageType := fmt.Sprintf(ws.MessageTypeConsole, params.Id)
	client, err = ws.NewClient(http,
		ws.WithMessageRecvHandler(messageType, func(recvMessage *ws.RecvMessage) {
			var cmd command
			err = json.Unmarshal(recvMessage.Message, &cmd)
			if err != nil {
				slog.Warn("console", "json unmarshal", err.Error())
			}
			if shell.Conn == nil {
				slog.Warn("console", "shell is nil", err.Error())
				return
			}
			if cmd.Content.Command != "" {
				_, err = shell.Conn.Write([]byte(cmd.Content.Command))
				if err != nil {
					slog.Warn("console", "shell read", err.Error())
				}
			}
			if cmd.Size.Height > 0 && cmd.Size.Width > 0 {
				err = docker.Sdk.Client.ContainerExecResize(client.CtxContext, execID, container.ResizeOptions{
					Height: uint(cmd.Size.Height),
					Width:  uint(cmd.Size.Width),
				})
				if err != nil {
					slog.Warn("console container tty", "resize", cmd.Size, "error", err)
				}
			}
		}),
	)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	clientReady := false
	defer func() {
		if !clientReady {
			_ = client.Close()
		}
	}()
	go func() {
		select {
		case <-client.CtxContext.Done():
			if shell.Conn != nil {
				_ = shell.CloseWrite()
				shell.Close()
			}
			return
		}
	}()

	execID, shell, err = docker.Sdk.ContainerExec(client.CtxContext, containerName, container.ExecOptions{
		Privileged:   true,
		Tty:          true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Cmd: []string{
			params.Cmd,
		},
		User: params.User,
		ConsoleSize: &[2]uint{
			params.Height, params.Width,
		},
		WorkingDir: params.WorkDir,
	})
	if err != nil {
		_ = notice.Message{}.Error(".consoleError", err.Error())
		self.JsonResponseWithError(http, err, 500)
		return
	}

	clientReady = true
	go client.ReadMessage()
	go func() {
		out := make([]byte, 2028)
		for {
			n, err := shell.Conn.Read(out)
			if err != nil {
				return
			}
			err = client.SendMessage(&ws.RespMessage{
				Type: messageType,
				Data: string(out[:n]),
			})
			if err != nil {
				slog.Warn("websocket shell write", "error", err.Error())
				return
			}
		}
	}()
}

func (self Home) WsSshConsole(http *gin.Context) {
	if !websocket.IsWebSocketUpgrade(http.Request) {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonUseWsConnect), 500)
		return
	}
	type ParamsValidate struct {
		Name   string `json:"name" uri:"name" binding:"required"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if params.Name == "" {
		params.Name = define.DockerDefaultClientName
	}
	var err error
	var sshClient *ssh.Client
	var read io.Reader
	var write io.WriteCloser
	var session *ssh2.Session

	messageType := fmt.Sprintf(ws.MessageTypeConsoleSsh, params.Name)
	client, err := ws.NewClient(http,
		ws.WithMessageRecvHandler(messageType, func(recvMessage *ws.RecvMessage) {
			var cmd command
			err = json.Unmarshal(recvMessage.Message, &cmd)
			if err != nil {
				slog.Warn("console", "json unmarshal", err.Error())
			}
			if cmd.Content.Command != "" {
				_, err = write.Write([]byte(cmd.Content.Command))
				if err != nil {
					slog.Warn("console", "json unmarshal", err.Error())
				}
			}
			if cmd.Size.Width > 0 && cmd.Size.Height > 0 {
				err = session.WindowChange(cmd.Size.Height, cmd.Size.Width)
				if err != nil {
					slog.Warn("console", "change size", cmd.Size, "err", err)
				}
			}
		}),
	)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	clientReady := false
	defer func() {
		if !clientReady {
			_ = client.Close()
		}
	}()

	err = func() error {
		dockerEnv, err := logic.Env{}.GetEnvByName(params.Name)
		if err != nil {
			return err
		}
		if !dockerEnv.EnableSSH || dockerEnv.SshServerInfo == nil {
			return function.ErrorMessage(define.ErrorMessageHomeWsHostConsoleSshNotSetting)
		}
		sshClient, err = ssh.NewClient(ssh.WithServerInfo(dockerEnv.SshServerInfo)...)
		if err != nil {
			return err
		}
		session, read, write, err = sshClient.NewPtySession(params.Height, params.Width)
		if err != nil {
			return err
		}
		return nil
	}()

	if err != nil {
		_ = client.SendMessage(&ws.RespMessage{
			Type: messageType,
			Data: err.Error(),
		})
		return
	}

	clientReady = true
	go func() {
		out := make([]byte, 2028)
		for {
			n, err := read.Read(out)
			if err != nil {
				return
			}
			err = client.SendMessage(&ws.RespMessage{
				Type: messageType,
				Data: string(out[:n]),
			})
			if err != nil {
				slog.Warn("websocket", "shell write", err.Error())
				return
			}
		}
	}()

	go func() {
		select {
		case <-client.CtxContext.Done():
			if sshClient != nil {
				sshClient.Close()
			}
			return
		}
	}()

	go client.ReadMessage()
}

func (self Home) WsShellConsole(http *gin.Context) {
	if !websocket.IsWebSocketUpgrade(http.Request) {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonUseWsConnect), 500)
		return
	}
	type ParamsValidate struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	var err error
	var read io.Reader
	var write io.WriteCloser
	var closeTerminalOnce sync.Once
	var resizeTerminal func(size *pty.Winsize) error

	client, err := ws.NewClient(http,
		ws.WithMessageRecvHandler(ws.MessageTypeConsoleShell, func(recvMessage *ws.RecvMessage) {
			var cmd command
			err = json.Unmarshal(recvMessage.Message, &cmd)
			if err != nil {
				slog.Warn("console", "json unmarshal", err.Error())
			}
			if cmd.Content.Command != "" {
				_, err = write.Write([]byte(cmd.Content.Command))
				if err != nil {
					slog.Warn("console shell write", "error", err.Error())
				}
			}
			if resizeTerminal != nil && cmd.Size.Width > 0 && cmd.Size.Height > 0 {
				err = resizeTerminal(&pty.Winsize{
					Rows: uint16(cmd.Size.Height),
					Cols: uint16(cmd.Size.Width),
				})
				if err != nil {
					slog.Warn("console shell resize", "size", cmd.Size, "error", err)
				}
			}
		}),
	)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	clientReady := false
	defer func() {
		if !clientReady {
			_ = client.Close()
		}
	}()

	localCmd, err := local.New(
		local.WithDefaultShell(),
		local.WithDefaultShellDir(),
		local.WithInteractiveTerminalEnv(),
		local.WithCtx(client.CtxContext),
		// pty.StartWithSize already sets Setsid and Setctty; adding Setpgid can make fork/exec fail with EPERM.
		local.WithKillProcessGroupOnCancel(),
	)
	if err != nil {
		_ = client.SendMessage(&ws.RespMessage{
			Type: ws.MessageTypeConsoleShell,
			Data: err.Error(),
		})
		return
	}
	read, write, err = localCmd.RunInTerminal(&pty.Winsize{
		Rows: uint16(params.Height),
		Cols: uint16(params.Width),
	})
	if err != nil {
		_ = client.SendMessage(&ws.RespMessage{
			Type: ws.MessageTypeConsoleShell,
			Data: err.Error(),
		})
		return
	}
	if resizer, ok := localCmd.(interface {
		ResizeTerminal(size *pty.Winsize) error
	}); ok {
		resizeTerminal = resizer.ResizeTerminal
	}

	closeTerminal := func() {
		closeTerminalOnce.Do(func() {
			if write != nil {
				_ = write.Close()
			}
		})
	}

	clientReady = true
	go func() {
		select {
		case <-client.CtxContext.Done():
			closeTerminal()
			return
		}
	}()

	go func() {
		defer closeTerminal()
		out := make([]byte, 2028)
		for {
			n, err := read.Read(out)
			if err != nil {
				return
			}
			err = client.SendMessage(&ws.RespMessage{
				Type: ws.MessageTypeConsoleShell,
				Data: string(out[:n]),
			})
			if err != nil {
				slog.Warn("websocket shell write", "error", err.Error())
				return
			}
		}
	}()

	go client.ReadMessage()
}

func (self Home) ConsoleLink(http *gin.Context) {
	params := accessor.ConsoleInstance{}
	if http.Request.ContentLength != 0 && !self.Validate(http, &params) {
		return
	}

	if params.Host == nil && params.Container == nil {
		logic.Setting{}.GetByKey(logic.SettingGroupSetting, logic.SettingGroupSettingConsoleInstance, &params)
	} else {
		if params.Host == nil {
			params.Host = make([]string, 0)
		}
		if params.Container == nil {
			params.Container = make([]string, 0)
		}
		err := logic.Setting{}.Save(&entity.Setting{
			GroupName: logic.SettingGroupSetting,
			Name:      logic.SettingGroupSettingConsoleInstance,
			Value: &accessor.SettingValueOption{
				ConsoleInstance: &params,
			},
		})
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	if params.Host == nil {
		params.Host = make([]string, 0)
	}
	if params.Container == nil {
		params.Container = make([]string, 0)
	}
	self.JsonResponseWithoutError(http, params)
}

func (self Home) Info(http *gin.Context) {
	dockerSdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if err = (statLogic.Stat{}).ReconcileSystemStat(dockerSdk); err != nil {
		slog.Warn("reconcile system stat", "dockerEnvName", dockerSdk.Name, "error", err)
	}
	info, err := dockerSdk.Client.Info(dockerSdk.Ctx)
	if err == nil && info.ID != "" {
		info.Name = fmt.Sprintf("%s - %s", dockerSdk.Name, dockerSdk.DockerEnv.Address)
	}
	var public string
	if v, ok := storage.Cache.Get(storage.CacheKeyRsaPub); ok {
		public = string(v.([]byte))
	}
	dpanelInfo := logic.Setting{}.GetDPanelInfo()
	var containerInfo gin.H
	if dpanelInfo.ContainerInfo.ContainerJSONBase != nil {
		containerInfo = gin.H{
			"Id":   dpanelInfo.ContainerInfo.ID,
			"Name": dpanelInfo.ContainerInfo.Name,
			"HostConfig": gin.H{
				"NetworkMode": dpanelInfo.ContainerInfo.HostConfig.NetworkMode,
			},
		}
	}

	dockerEnv := *dockerSdk.DockerEnv
	dockerEnv.SshServerInfo = nil
	dockerEnv.TlsCert = ""
	dockerEnv.TlsCa = ""
	dockerEnv.TlsKey = ""

	dpanelInfoResult := function.StructToMap(dpanelInfo)
	dpanelInfoResult["containerInfo"] = containerInfo
	loginSetting := logic.Setting{}.GetLoginSetting()
	if loginSetting.SystemEntrance != nil {
		dpanelInfoResult["systemEntrance"] = loginSetting.SystemEntrance
	}
	var founder gin.H
	if founderSetting, _ := dao.Setting.
		Where(dao.Setting.GroupName.Eq(logic.SettingGroupUser)).
		Where(dao.Setting.Name.Eq(logic.SettingGroupUserFounder)).First(); founderSetting != nil && founderSetting.Value != nil {
		founder = gin.H{
			"username": founderSetting.Value.Username,
			"password": function.MaskSensitiveValue(founderSetting.Value.Password),
		}
	}

	result := gin.H{
		"info":          info,
		"clientVersion": dockerSdk.Client.ClientVersion(),
		"sdkVersion":    api.DefaultVersion,
		"dpanel":        dpanelInfoResult,
		"dockerEnv":     dockerEnv,
		"plugin":        plugin.Wrapper{}.GetPluginList(),
		"rsa": gin.H{
			"public": public,
		},
	}
	if founder != nil {
		result["founder"] = founder
	}
	self.JsonResponseWithoutError(http, result)
	return
}

func (self Home) Usage(http *gin.Context) {
	sdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	// 有些设备的docker获取磁盘占用比较耗时，跑一下后台协程去获取数据
	go func() {
		progress, owner, err := ws.NewFdProgressPip(http, sdk.Name, ws.MessageTypeDiskUsage)
		if err != nil {
			slog.Warn("create disk usage progress", "dockerEnvName", sdk.Name, "error", err)
			return
		}
		if !owner {
			return
		}
		defer progress.Close()
		// 20 分种后强制终止
		ctx, cancel := context.WithTimeout(sdk.Ctx, time.Minute*20)
		defer cancel()
		stopCancel := context.AfterFunc(progress.Context(), cancel)
		defer stopCancel()

		result := accessor.DiskUsage{DockerEnvName: sdk.Name}
		type dockerUsageResult struct {
			usage types.DiskUsage
			err   error
		}
		type hostUsageResult struct {
			usage accessor.SystemDiskUsage
			err   error
		}
		dockerResult := make(chan dockerUsageResult, 1)
		go func() {
			usage, collectErr := (statLogic.Stat{}).DockerDiskUsage(ctx, sdk)
			dockerResult <- dockerUsageResult{usage: usage, err: collectErr}
		}()

		var hostResult chan hostUsageResult
		if sdk.DockerEnv.EnableSystemStat {
			hostResult = make(chan hostUsageResult, 1)
			go func() {
				usage, collectErr := (statLogic.Stat{}).HostDiskUsage(ctx, sdk)
				hostResult <- hostUsageResult{usage: usage, err: collectErr}
			}()
		}

		dockerSucceeded := false
		completedTotal := 1
		if hostResult != nil {
			completedTotal++
		}
		for completed := 0; completed < completedTotal; completed++ {
			select {
			case collected := <-dockerResult:
				if collected.err != nil {
					slog.Warn("collect Docker disk usage", "dockerEnvName", sdk.Name, "error", collected.err)
					continue
				}
				result.Docker = collected.usage
				dockerSucceeded = true
			case collected := <-hostResult:
				if collected.err != nil {
					slog.Warn("collect system disk usage", "dockerEnvName", sdk.Name, "error", collected.err)
					continue
				}
				result.System = &collected.usage
			case <-ctx.Done():
				slog.Warn("collect disk usage", "dockerEnvName", sdk.Name, "error", ctx.Err())
				return
			}
		}
		if !dockerSucceeded {
			return
		}
		result.UpdatedAt = time.Now()
		_ = logic.Setting{}.Save(&entity.Setting{
			GroupName: logic.SettingGroupSetting,
			Name:      logic.SettingGroupSettingDiskUsage,
			Value: &accessor.SettingValueOption{
				DiskUsage: &result,
			},
		})

		time.Sleep(time.Second * 3)
		progress.BroadcastMessage(&result)
	}()

	diskUsage := accessor.DiskUsage{}
	logic.Setting{}.GetByKey(logic.SettingGroupSetting, logic.SettingGroupSettingDiskUsage, &diskUsage)
	if diskUsage.DockerEnvName != sdk.Name {
		// 用量统计如果不是当前环境的变清空掉，等待获取
		diskUsage = accessor.DiskUsage{}
	}

	containerRunningTotal := struct {
		Stop      int `json:"stop"`
		Pause     int `json:"pause"`
		Unhealthy int `json:"unhealthy"`
	}{
		Stop:      0,
		Pause:     0,
		Unhealthy: 0,
	}

	var containerList []container.Summary

	if containerList, err = sdk.Client.ContainerList(sdk.Ctx, container.ListOptions{
		All: true,
	}); err == nil {
		containerLogic := applicationLogic.Container{}
		for _, item := range containerList {
			unhealthy := false
			if item.State == string(container.StateExited) {
				containerRunningTotal.Stop += 1
			}
			if item.State == string(container.StatePaused) {
				containerRunningTotal.Pause += 1
			}
			if strings.Contains(item.Status, "unhealthy") {
				containerRunningTotal.Unhealthy += 1
				unhealthy = true
			}
			if strings.Contains(item.Status, "Restarting") {
				containerRunningTotal.Unhealthy += 1
				unhealthy = true
			}
			var containerInfo container.InspectResponse
			var inspectInfo *container.InspectResponse
			if info, err := sdk.Client.ContainerInspect(sdk.Ctx, item.ID); err == nil {
				containerInfo = info
				inspectInfo = &containerInfo
			}
			if !unhealthy && containerLogic.RuntimeStatus(applicationLogic.ContainerRuntimeItem{
				Summary: item,
				Inspect: inspectInfo,
			}).Unhealthy {
				containerRunningTotal.Unhealthy += 1
			}
		}
	}

	networkRow, _ := sdk.Client.NetworkList(sdk.Ctx, network.ListOptions{})
	recycleQuery := dao.Site.Where(dao.Site.DeletedAt.IsNotNull()).Unscoped().Where(gen.Cond(
		datatypes.JSONQuery("env").Equals(sdk.Name, "dockerEnvName"),
	)...)
	if containerList != nil {
		names := make([]string, 0)
		for _, summary := range containerList {
			for _, name := range summary.Names {
				names = append(names, strings.TrimPrefix(name, "/"))
			}
		}
		recycleQuery = recycleQuery.Where(dao.Site.SiteName.NotIn(names...))
	}
	containerTask, _ := recycleQuery.Count()
	imageTask, _ := dao.Image.Where(dao.Image.Setting.IsNotNull()).Count()
	backupData, _ := dao.Backup.Count()

	self.JsonResponseWithoutError(http, gin.H{
		"diskUsage": diskUsage,
		"total": map[string]interface{}{
			"network":          len(networkRow),
			"containerTask":    int(containerTask),
			"containerRunning": containerRunningTotal,
			"imageTask":        int(imageTask),
			"backup":           int(backupData),
		},
	})
}

func (self Home) GetStatList(http *gin.Context) {
	type runtimeStat struct {
		DockerEnvName string                `json:"dockerEnvName"`
		Docker        []*stats.Usage        `json:"docker"`
		System        *statLogic.SystemStat `json:"system,omitempty"`
	}
	type ParamsValidate struct {
		Follow bool `json:"follow"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	sdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	var progress *ws.ProgressPip
	var progressDone <-chan struct{}
	if params.Follow {
		var owner bool
		progress, owner, err = ws.NewFdProgressPip(http, sdk.Name, ws.MessageTypeContainerAllStat)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		if !owner {
			self.JsonResponseWithoutError(http, runtimeStat{
				DockerEnvName: sdk.Name,
				Docker:        make([]*stats.Usage, 0),
			})
			return
		}
		defer progress.Close()
		closeTimer := time.AfterFunc(time.Hour, progress.Close)
		defer closeTimer.Stop()
		progressDone = progress.Done()
	}

	// Docker 与系统统计独立采样，发送时使用各自最近一帧。
	readerCtx, cancel := context.WithCancel(sdk.Ctx)
	defer cancel()
	dockerData := (statLogic.Stat{}).ReadDockerStat(readerCtx, sdk)

	var systemData <-chan statLogic.SystemStatFrame
	var systemRetry <-chan time.Time
	if sdk.DockerEnv.EnableSystemStat {
		systemData = (statLogic.Stat{}).ReadSystemStat(readerCtx, sdk)
	}

	// 固定间隔组装快照；普通请求返回一次，WebSocket 持续广播。
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	var latestDocker statLogic.DockerStatFrame
	var latestSystem *statLogic.SystemStatFrame
	for {
		select {
		case <-http.Request.Context().Done():
			return
		case <-sdk.Ctx.Done():
			return
		case <-progressDone:
			slog.Debug("home get stat list progress done")
			return
		case value, ok := <-dockerData:
			if !ok {
				dockerData = nil
				continue
			}
			latestDocker = value
		case value, ok := <-systemData:
			if !ok {
				systemData = nil
				latestSystem = nil
				if params.Follow {
					systemRetry = time.After(3 * time.Second)
				}
				continue
			}
			latestSystem = &value
		case <-systemRetry:
			systemRetry = nil
			if progress.Context().Err() != nil {
				return
			}
			systemData = (statLogic.Stat{}).ReadSystemStat(readerCtx, sdk)
		case <-ticker.C:
			result := runtimeStat{
				DockerEnvName: sdk.Name,
				Docker:        make([]*stats.Usage, 0, len(latestDocker)),
			}
			var systemContainers map[string]statLogic.SystemContainerStat
			if latestSystem != nil {
				system := latestSystem.System
				result.System = &system
				systemContainers = latestSystem.Containers
			}
			// 系统采样仅修正 Sysbox 容器中明确可用的指标，其余字段保留 Docker 原值。
			for _, item := range latestDocker {
				if item == nil {
					continue
				}
				usage := *item
				if systemUsage, exists := systemContainers[item.Container]; exists && systemUsage.Usage != nil {
					if systemUsage.HasCPU {
						usage.Cpu = systemUsage.Usage.Cpu
						usage.CPUThrottled = systemUsage.Usage.CPUThrottled
					}
					if systemUsage.HasMemory {
						usage.Memory = systemUsage.Usage.Memory
					}
					if systemUsage.HasBlockIO {
						usage.BlockIO = systemUsage.Usage.BlockIO
						usage.BlockTotal = systemUsage.Usage.BlockTotal
						usage.BlockIOWaiting = systemUsage.Usage.BlockIOWaiting
					}
				}
				result.Docker = append(result.Docker, &usage)
			}
			if !params.Follow {
				self.JsonResponseWithoutError(http, result)
				return
			}
			progress.BroadcastMessage(result)
		}
	}
}

func (self Home) Reset(http *gin.Context) {
	type ParamsValidate struct {
		Entrance   *string `json:"entrance"`
		Cache      bool    `json:"cache"`
		OnlineUser bool    `json:"onlineUser"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	total := 0
	var eventTotal int64
	var noticeTotal int64

	if params.Entrance != nil {
		setting, _ := (logic.Setting{}).GetValue(logic.SettingGroupSetting, logic.SettingGroupSettingLogin)
		if setting == nil {
			setting = &entity.Setting{GroupName: logic.SettingGroupSetting, Name: logic.SettingGroupSettingLogin, Value: &accessor.SettingValueOption{Login: &accessor.Login{}}}
		}
		if setting.Value == nil {
			setting.Value = &accessor.SettingValueOption{}
		}
		if setting.Value.Login == nil {
			setting.Value.Login = &accessor.Login{}
		}
		entrance := strings.Trim(*params.Entrance, "/")
		setting.Value.Login.SystemEntrance = &accessor.SystemEntrance{
			Entrance: &entrance,
			Enable:   entrance != "",
		}
		if err := (logic.Setting{}).Save(setting); err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	if params.Cache {
		cacheKeys := []string{
			storage.CacheKeyLoginFailed,
			storage.CacheKeyOauthCode,
			storage.CacheKeyContainerUpgradeCheck,
			storage.CacheKeyContainerUpgradeLogs,
			storage.CacheKeyImageRootFs,
			storage.CacheKeyDockerEvents,
			storage.CacheKeyDockerContainerPort,
			storage.CacheKeyDockerContainerRuntime,
			storage.CacheKeyAttach,
			storage.CacheKeyAsset,
		}
		for key := range storage.Cache.Items() {
			if _, ok := function.IndexArrayWalk(cacheKeys, func(pattern string) bool {
				if index := strings.Index(pattern, "%s"); index >= 0 {
					return strings.HasPrefix(key, pattern[:index])
				}
				return key == pattern
			}); ok {
				storage.Cache.Delete(key)
			}
		}
	}

	if params.Cache {
		if notices, err := dao.Notice.Find(); err == nil {
			noticeTotal = int64(len(notices))
		}
		db, err := facade.GetDbFactory().Channel("default")
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		if err = db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&entity.Notice{}).Error; err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		_ = db.Exec("vacuum").Error
	}
	if params.Cache {
		if _, err := os.Stat(storage.Local{}.GetLocalTempDir()); err == nil {
			if err = os.RemoveAll(storage.Local{}.GetLocalTempDir()); err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
		}
	}
	runtime.GC()
	debug.FreeOSMemory()

	if params.OnlineUser {
		storage.Cache.Set(storage.CacheKeyCommonServerStartTime, time.Now().Add(time.Second).Truncate(time.Second), cache.NoExpiration)
		if value, exists := http.Get("userInfo"); exists {
			if userInfo, ok := value.(logic.UserInfo); ok {
				ws.GetCollect().LeaveByUserId(userInfo.UserId)
			}
		}
	}

	self.JsonResponseWithoutError(http, gin.H{
		"gc":     true,
		"temp":   total,
		"events": eventTotal,
		"notice": noticeTotal,
	})
}
