package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/exec"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/plugin"
	"github.com/donknap/dpanel/common/service/registry"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/mcuadros/go-version"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"io"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Home struct {
	controller.Abstract
}

func (self Home) Index(ctx *gin.Context) {
	self.JsonResponseWithoutError(ctx, "hello world!")
	return
}

func (self Home) WsNotice(http *gin.Context) {
	if !websocket.IsWebSocketUpgrade(http.Request) {
		self.JsonResponseWithError(http, errors.New("please connect using websocket"), 500)
		return
	}

	client, err := ws.NewClient(http, ws.ClientOption{})
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
		slog.Error("websocket", "connect", err.Error())
	}
}

func (self Home) WsConsole(http *gin.Context) {
	if !websocket.IsWebSocketUpgrade(http.Request) {
		self.JsonResponseWithError(http, errors.New("please connect using websocket"), 500)
		return
	}
	type ParamsValidate struct {
		Id      string `uri:"id" binding:"required"`
		Width   uint   `form:"width"`
		Height  uint   `form:"height"`
		Cmd     string `form:"cmd,default=/bin/sh"`
		WorkDir string `form:"workDir"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	if params.Id == "" {
		self.JsonResponseWithError(http, errors.New("请指定容器Id"), 500)
		return
	}
	containerName := params.Id
	if _, pluginName, exists := strings.Cut(params.Id, ":"); exists {
		containerName = pluginName
	}
	if params.WorkDir == "" {
		params.WorkDir = "/"
	}
	out, err := docker.Sdk.Client.ContainerExecCreate(docker.Sdk.Ctx, containerName, container.ExecOptions{
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
	shell, err := docker.Sdk.Client.ContainerExecAttach(docker.Sdk.Ctx, out.ID, container.ExecStartOptions{
		Tty: true,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	type command struct {
		Type    string `json:"type"`
		Content struct {
			Command string `json:"command"`
		} `json:"content"`
	}
	client, err := ws.NewClient(http, ws.ClientOption{
		CloseHandler: func() {
			if _, pluginName, exists := strings.Cut(params.Id, ":"); exists {
				_ = notice.Message{}.Info(".consoleDestroyPlugin", pluginName)

				if webShellPlugin, err := plugin.NewPlugin(plugin.PluginWebShell, map[string]*plugin.TemplateParser{
					"webshell": {
						ContainerName: pluginName,
					},
				}); err == nil {
					_ = webShellPlugin.Destroy()
				}
			}
			shell.Close()
		},
		RecvMessageHandler: map[string]ws.RecvMessageHandlerFn{
			"console": func(recvMessage *ws.RecvMessage) {
				var cmd command
				err = json.Unmarshal(recvMessage.Message, &cmd)
				if err != nil {
					slog.Error("console", "json unmarshal", err.Error())
				}
				_, err = shell.Conn.Write([]byte(cmd.Content.Command))
				if err != nil {
					slog.Error("console", "shell read", err.Error())
				}
			},
		},
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	lock := sync.RWMutex{}
	go client.ReadMessage()
	go func() {
		out := make([]byte, 2028)
		for {
			n, err := shell.Conn.Read(out)
			if err != nil {
				return
			}
			processedOutput := string(out[:n])
			lock.Lock()
			err = client.Conn.WriteMessage(websocket.TextMessage, ws.RespMessage{
				Type: fmt.Sprintf(ws.MessageTypeConsole, params.Id),
				Data: processedOutput,
			}.ToJson())
			lock.Unlock()
			if err != nil {
				slog.Error("websocket", "shell write", err.Error())
			}
		}
	}()
}

func (self Home) Info(http *gin.Context) {
	startTime := time.Now()
	dpanelContainerInfo := container.InspectResponse{}
	new(logic.Setting).GetByKey(logic.SettingGroupSetting, logic.SettingGroupSettingDPanelInfo, &dpanelContainerInfo)
	slog.Debug("info time", "use", time.Now().Sub(startTime).String())

	startTime = time.Now()
	info, _ := docker.Sdk.Client.Info(docker.Sdk.Ctx)
	if info.ID != "" {
		info.Name = fmt.Sprintf("%s - %s", docker.Sdk.Name, docker.Sdk.Client.DaemonHost())
	}
	slog.Debug("info time", "use", time.Now().Sub(startTime).String())

	self.JsonResponseWithoutError(http, gin.H{
		"info":       info,
		"sdkVersion": docker.Sdk.Client.ClientVersion(),
		"dpanel": map[string]interface{}{
			"version":       facade.GetConfig().GetString("app.version"),
			"family":        facade.GetConfig().GetString("app.family"),
			"env":           facade.GetConfig().GetString("app.env"),
			"containerInfo": dpanelContainerInfo,
		},
		"plugin": plugin.Wrapper{}.GetPluginList(),
	})
	return
}

func (self Home) CheckNewVersion(http *gin.Context) {
	var tags []string
	var err error
	currentVersion := facade.GetConfig().GetString("app.version")
	newVersion := ""

	for _, s := range []string{
		"registry.cn-hangzhou.aliyuncs.com",
	} {
		option := make([]registry.Option, 0)
		option = append(option, registry.WithRequestCacheTime(time.Hour*24))
		option = append(option, registry.WithRegistryHost(s))
		reg := registry.New(option...)
		tags, err = reg.Repository.GetImageTagList("dpanel/dpanel")
		if err == nil {
			break
		}
	}

	for _, ver := range tags {
		if !strings.Contains(ver, "-") && version.Compare(ver, currentVersion, ">") {
			newVersion = ver
			break
		}
	}

	self.JsonResponseWithoutError(http, gin.H{
		"version":    currentVersion,
		"newVersion": newVersion,
	})
	return
}

func (self Home) Usage(http *gin.Context) {
	// 有些设备的docker获取磁盘占用比较耗时，跑一下后台协程去获取数据
	go func() {
		diskUsage, err := docker.Sdk.Client.DiskUsage(docker.Sdk.Ctx, types.DiskUsageOptions{
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
						Usage:     &diskUsage,
						UpdatedAt: time.Now(),
					},
				},
			})
		}
		return
	}()

	diskUsage := accessor.DiskUsage{
		Usage: &types.DiskUsage{},
	}
	logic.Setting{}.GetByKey(logic.SettingGroupSetting, logic.SettingGroupSettingDiskUsage, &diskUsage)
	type portItem struct {
		Port docker.PortItem `json:"port"`
		Name string          `json:"name"`
	}
	ports := make([]*portItem, 0)
	containerList, err := docker.Sdk.Client.ContainerList(docker.Sdk.Ctx, container.ListOptions{
		All: true,
	})
	containerRunningTotal := struct {
		Stop      int `json:"stop"`
		Pause     int `json:"pause"`
		Unhealthy int `json:"unhealthy"`
	}{
		Stop:      0,
		Pause:     0,
		Unhealthy: 0,
	}
	if err == nil {
		for _, item := range containerList {
			if item.State == "exited" {
				containerRunningTotal.Stop += 1
			}
			if item.State == "paused" {
				containerRunningTotal.Pause += 1
			}
			if strings.Contains(item.Status, "unhealthy") {
				containerRunningTotal.Unhealthy += 1
			}
			usePort := make([]*portItem, 0)
			if item.HostConfig.NetworkMode == "host" {
				imageInfo, err := docker.Sdk.Client.ImageInspect(docker.Sdk.Ctx, item.ImageID)
				if err == nil {
					for port := range imageInfo.Config.ExposedPorts {
						usePort = append(usePort, &portItem{
							Name: item.Names[0],
							Port: docker.PortItem{
								Host:   port.Port(),
								Dest:   port.Port(),
								HostIp: "0.0.0.0",
							},
						})
					}
				}
			} else {
				for _, port := range item.Ports {
					if port.PublicPort == 0 {
						continue
					}
					usePort = append(usePort, &portItem{
						Name: item.Names[0],
						Port: docker.PortItem{
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
	containerTask, _ := dao.Site.Where(dao.Site.DeletedAt.IsNotNull()).Unscoped().Count()
	imageTask, _ := dao.Image.Count()
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
	command := []string{
		"stats", "-a",
		"--format", "json",
	}
	option := make([]exec.Option, 0)
	if !params.Follow {
		ctx, cancel := context.WithCancel(context.Background())
		defer func() {
			cancel()
		}()
		command = append(command, "--no-stream")
		option = append(option, docker.Sdk.GetRunCmd(command...)...)
		option = append(option, exec.WithCtx(ctx))
		cmd, err := exec.New(option...)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		list, err := logic.Stat{}.GetStat(cmd.RunWithResult())
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		self.JsonResponseWithoutError(http, gin.H{
			"list": list,
		})
		return
	}

	progress, err := ws.NewFdProgressPip(http, ws.MessageTypeContainerStat)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	if progress.IsShadow() {
		self.JsonResponseWithoutError(http, gin.H{
			"list": "",
		})
		return
	}
	defer progress.Close()
	lastSendTime := time.Now()
	progress.OnWrite = func(p string) error {
		if p == "" {
			return nil
		}
		list, err := logic.Stat{}.GetStat(p)
		if err != nil {
			return err
		}
		if function.IsEmptyArray(list) {
			return nil
		}
		if time.Now().Sub(lastSendTime) > 2*time.Second {
			progress.BroadcastMessage(list)
			lastSendTime = time.Now()
		}
		return nil
	}

	option = append(option, docker.Sdk.GetRunCmd(command...)...)
	option = append(option, exec.WithCtx(progress.Context()))
	cmd, err := exec.New(option...)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	out, err := cmd.RunInPip()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	_, err = io.Copy(progress, out)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	// 等待进程退出
	err = cmd.Cmd().Wait()
	if err != nil {
		slog.Warn("home stat wait process", "error", err)
	}
	self.JsonResponseWithoutError(http, gin.H{
		"list": "",
	})
	return
}

func (self Home) UpgradeScript(http *gin.Context) {
	dpanelContainerInfo := container.InspectResponse{}
	if exists := new(logic.Setting).GetByKey(logic.SettingGroupSetting, logic.SettingGroupSettingDPanelInfo, &dpanelContainerInfo); !exists {
		self.JsonResponseWithError(http, notice.Message{}.New(".systemUpgradeDPanelNotFound"), 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"info": dpanelContainerInfo,
	})
	return
}
