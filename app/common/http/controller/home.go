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
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	applicationLogic "github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	types2 "github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/exec/local"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/plugin"
	"github.com/donknap/dpanel/common/service/ssh"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	ssh2 "golang.org/x/crypto/ssh"
	"gorm.io/datatypes"
	"gorm.io/gen"
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

	client, err := ws.NewClient(http)
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
		Width   uint   `json:"width"`
		Height  uint   `json:"height"`
		Cmd     string `json:"cmd"`
		WorkDir string `json:"workDir"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	params.Cmd = strings.TrimSpace(params.Cmd)
	if params.Cmd == "" {
		params.Cmd = "/bin/sh"
	}
	if params.Cmd == "bin/sh" || params.Cmd == "bin/bash" {
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
	var out container.ExecCreateResponse

	messageType := fmt.Sprintf(ws.MessageTypeConsole, params.Id)
	client, err := ws.NewClient(http,
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
				err = docker.Sdk.Client.ContainerExecResize(docker.Sdk.Ctx, out.ID, container.ResizeOptions{
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

	out, err = docker.Sdk.Client.ContainerExecCreate(client.CtxContext, containerName, container.ExecOptions{
		Privileged:   true,
		Tty:          true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Cmd: []string{
			params.Cmd,
		},
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
	shell, err = docker.Sdk.Client.ContainerExecAttach(client.CtxContext, out.ID, container.ExecStartOptions{
		Tty: true,
	})

	if err != nil {
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
	info, err := docker.Sdk.Client.Info(docker.Sdk.Ctx)
	if err == nil && info.ID != "" {
		info.Name = fmt.Sprintf("%s - %s", docker.Sdk.Name, docker.Sdk.DockerEnv.Address)
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

	dockerEnv := *docker.Sdk.DockerEnv
	dockerEnv.SshServerInfo = nil
	dockerEnv.TlsCert = ""
	dockerEnv.TlsCa = ""
	dockerEnv.TlsKey = ""

	dpanelInfoResult := function.StructToMap(dpanelInfo)
	dpanelInfoResult["containerInfo"] = containerInfo

	self.JsonResponseWithoutError(http, gin.H{
		"info":          info,
		"clientVersion": docker.Sdk.Client.ClientVersion(),
		"sdkVersion":    api.DefaultVersion,
		"dpanel":        dpanelInfoResult,
		"dockerEnv":     dockerEnv,
		"plugin":        plugin.Wrapper{}.GetPluginList(),
		"rsa": gin.H{
			"public": public,
		},
	})
	return
}

func (self Home) Usage(http *gin.Context) {
	// 有些设备的docker获取磁盘占用比较耗时，跑一下后台协程去获取数据
	go func() {
		progress, err := ws.NewFdProgressPip(http, ws.MessageTypeDiskUsage)
		defer func() {
			progress.Close()
		}()
		// 20 分种后强制终止
		ctx, _ := context.WithTimeout(docker.Sdk.Ctx, time.Minute*20)
		diskUsage, err := docker.Sdk.Client.DiskUsage(ctx, types.DiskUsageOptions{
			Types: []types.DiskUsageObject{
				types.ContainerObject,
				types.ImageObject,
				types.VolumeObject,
				types.BuildCacheObject,
			},
		})
		if err == nil {
			// 去掉无用的信息
			for i := range diskUsage.Containers {
				diskUsage.Containers[i].Labels = make(map[string]string)
			}
			for i := range diskUsage.Images {
				diskUsage.Images[i].Labels = make(map[string]string)
			}
			for i := range diskUsage.Volumes {
				diskUsage.Volumes[i].Labels = make(map[string]string)
			}
			if !function.IsEmptyArray(diskUsage.Images) {
				sort.Slice(diskUsage.Images, func(i, j int) bool {
					return diskUsage.Images[i].Size > diskUsage.Images[j].Size
				})
			}
			if !function.IsEmptyArray(diskUsage.Containers) {
				sort.Slice(diskUsage.Containers, func(i, j int) bool {
					return diskUsage.Containers[i].SizeRw+diskUsage.Containers[i].SizeRootFs > diskUsage.Containers[j].SizeRw+diskUsage.Containers[j].SizeRootFs
				})
			}

			if !function.IsEmptyArray(diskUsage.Volumes) {
				sort.Slice(diskUsage.Volumes, func(i, j int) bool {
					if diskUsage.Volumes[i].UsageData != nil && diskUsage.Volumes[j].UsageData != nil {
						return diskUsage.Volumes[i].UsageData.Size > diskUsage.Volumes[j].UsageData.Size
					}
					return false
				})
			}

			_ = logic.Setting{}.Save(&entity.Setting{
				GroupName: logic.SettingGroupSetting,
				Name:      logic.SettingGroupSettingDiskUsage,
				Value: &accessor.SettingValueOption{
					DiskUsage: &accessor.DiskUsage{
						DockerEnvName: docker.Sdk.Name,
						Usage:         &diskUsage,
						UpdatedAt:     time.Now(),
					},
				},
			})

			time.Sleep(time.Second * 3)
			progress.BroadcastMessage(&accessor.DiskUsage{
				Usage:     &diskUsage,
				UpdatedAt: time.Now(),
			})
		}
		return
	}()

	diskUsage := accessor.DiskUsage{
		Usage: &types.DiskUsage{},
	}
	logic.Setting{}.GetByKey(logic.SettingGroupSetting, logic.SettingGroupSettingDiskUsage, &diskUsage)
	if diskUsage.DockerEnvName != docker.Sdk.Name {
		// 用量统计如果不是当前环境的变清空掉，等待获取
		diskUsage = accessor.DiskUsage{
			Usage: &types.DiskUsage{},
		}
	}

	type portItem struct {
		Port types2.PortItem `json:"port"`
		Name string          `json:"name"`
	}
	ports := make([]*portItem, 0)

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
	var err error

	if containerList, err = docker.Sdk.Client.ContainerList(docker.Sdk.Ctx, container.ListOptions{
		All: true,
	}); err == nil {
		containerLogic := applicationLogic.Container{}
		for _, item := range containerList {
			unhealthy := false
			if item.State == "exited" {
				containerRunningTotal.Stop += 1
			}
			if item.State == "paused" {
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
			containerInspectOK := false
			if info, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, item.ID); err == nil {
				containerInfo = info
				inspectInfo = &containerInfo
				containerInspectOK = true
			}
			if !unhealthy && containerLogic.RuntimeStatus(applicationLogic.ContainerRuntimeItem{
				Summary: item,
				Inspect: inspectInfo,
			}).Unhealthy {
				containerRunningTotal.Unhealthy += 1
			}
			usePort := make([]*portItem, 0)
			if function.IsEmptyArray(item.Ports) {
				if containerInspectOK && containerInfo.HostConfig != nil && !function.IsEmptyMap(containerInfo.HostConfig.PortBindings) {
					for port, bindings := range containerInfo.HostConfig.PortBindings {
						for _, binding := range bindings {
							hostPort, _ := strconv.Atoi(binding.HostPort)
							if binding.HostIP == "" {
								binding.HostIP = "0.0.0.0"
							}
							usePort = append(usePort, &portItem{
								Name: item.Names[0],
								Port: types2.PortItem{
									Host:     strconv.Itoa(hostPort),
									Dest:     strconv.Itoa(port.Int()),
									HostIp:   binding.HostIP,
									Protocol: port.Proto(),
								},
							})
						}
					}
				}
			} else {
				for _, port := range item.Ports {
					if port.PublicPort == 0 {
						continue
					}
					usePort = append(usePort, &portItem{
						Name: item.Names[0],
						Port: types2.PortItem{
							Host:   strconv.Itoa(int(port.PublicPort)),
							Dest:   strconv.Itoa(int(port.PrivatePort)),
							HostIp: port.IP,
						},
					})
				}
			}
			ports = append(ports, usePort...)
		}
		sort.Slice(ports, func(i, j int) bool {
			return ports[i].Port.Host < ports[j].Port.Host
		})
	}

	networkRow, _ := docker.Sdk.Client.NetworkList(docker.Sdk.Ctx, network.ListOptions{})
	recycleQuery := dao.Site.Where(dao.Site.DeletedAt.IsNotNull()).Unscoped().Where(gen.Cond(
		datatypes.JSONQuery("env").Equals(docker.Sdk.Name, "dockerEnvName"),
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
			"port":             len(ports),
		},
		"port": ports,
	})
}

func (self Home) GetStatList(http *gin.Context) {
	type ParamsValidate struct {
		Follow bool `json:"follow"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	var err error

	if !params.Follow {
		list, err := docker.Sdk.ContainerStatsOneShot(docker.Sdk.Ctx)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		self.JsonResponseWithoutError(http, gin.H{
			"list": list,
		})
		return
	}

	progress, err := ws.NewFdProgressPip(http, ws.MessageTypeContainerAllStat)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	closeTimer := time.AfterFunc(time.Hour, func() {
		progress.Close()
	})

	if progress.IsShadow() {
		self.JsonResponseWithoutError(http, gin.H{
			"list": "",
		})
		return
	}
	defer progress.Close()

	out, err := docker.Sdk.ContainerStats(progress.Context(), types2.ContainerStatsOption{
		Stream: true,
	})
	if err != nil {
		slog.Debug("home get stat list", "error", err)
		self.JsonResponseWithoutError(http, gin.H{
			"list": "",
		})
		return
	}

	for {
		select {
		case <-docker.Sdk.Ctx.Done():
			progress.Close()
		case <-progress.Done():
			slog.Debug("home get stat list progress done")
			closeTimer.Stop()
			self.JsonResponseWithoutError(http, gin.H{
				"list": "",
			})
			return
		case list, ok := <-out:
			if !ok {
				// 关闭通道继续执行，正常回收资源
				progress.Close()
				continue
			}
			progress.BroadcastMessage(list)
		}
	}
}

func (self Home) Prune(http *gin.Context) {
	type ParamsValidate struct {
		EnableNotice   bool `json:"enableNotice"`
		EnableTempFile bool `json:"enableTempFile"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	total := 0
	var eventTotal int64
	var noticeTotal int64

	if params.EnableNotice {
		if oldRow, _ := dao.Notice.Last(); oldRow != nil {
			query := dao.Notice.Where(dao.Notice.ID.Lte(oldRow.ID))
			noticeTotal, _ = query.Count()
			_, _ = query.Delete()
		}
		if db, err := facade.GetDbFactory().Channel("default"); err == nil {
			db.Exec("vacuum")
		}
	}

	if params.EnableTempFile {
		_ = os.RemoveAll(storage.Local{}.GetLocalTempDir())
	}

	runtime.GC()
	debug.FreeOSMemory()

	self.JsonResponseWithoutError(http, gin.H{
		"gc":     true,
		"temp":   total,
		"events": eventTotal,
		"notice": noticeTotal,
	})
	return
}
