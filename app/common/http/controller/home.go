package controller

import (
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
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/plugin"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"log/slog"
	"sort"
	"strconv"
	"strings"
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
		Data: []byte(client.Fd),
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
	if params.WorkDir == "" {
		params.WorkDir = "/"
	}
	exec, err := docker.Sdk.Client.ContainerExecCreate(docker.Sdk.Ctx, params.Id, container.ExecOptions{
		Privileged:   true,
		Tty:          true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Cmd: []string{
			params.Cmd,
		},
		WorkingDir: params.WorkDir,
	})
	if err != nil {
		notice.Message{}.Error("console", err.Error())
		self.JsonResponseWithError(http, err, 500)
		return
	}
	shell, err := docker.Sdk.Client.ContainerExecAttach(docker.Sdk.Ctx, exec.ID, container.ExecStartOptions{
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
	go client.ReadMessage()
	go func() {
		out := make([]byte, 2028)
		for {
			n, err := shell.Conn.Read(out)
			if err != nil {
				return
			}
			processedOutput := string(out[:n])
			err = client.Conn.WriteMessage(websocket.TextMessage, ws.RespMessage{
				Type: fmt.Sprintf(ws.MessageTypeConsole, params.Id),
				Data: processedOutput,
			}.ToJson())
			if err != nil {
				slog.Error("websocket", "shell write", err.Error())
			}
		}
	}()
}

func (self Home) Info(http *gin.Context) {
	dpanelContainerInfo, _ := docker.Sdk.ContainerInfo(facade.GetConfig().GetString("app.name"))
	info, err := docker.Sdk.Client.Info(docker.Sdk.Ctx)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	info.Name = fmt.Sprintf("%s - %s", docker.Sdk.Host, docker.Sdk.Client.DaemonHost())
	initUser := false
	founder, err := logic.Setting{}.GetValue(logic.SettingGroupUser, logic.SettingGroupUserFounder)
	if err != nil || founder == nil {
		initUser = true
	}

	self.JsonResponseWithoutError(http, gin.H{
		"info":       info,
		"sdkVersion": docker.Sdk.Client.ClientVersion(),
		"initUser":   initUser,
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
			for i, _ := range diskUsage.Containers {
				diskUsage.Containers[i].Labels = make(map[string]string)
			}
			for i, _ := range diskUsage.Images {
				diskUsage.Images[i].Labels = make(map[string]string)
			}
			for i, _ := range diskUsage.Volumes {
				diskUsage.Volumes[i].Labels = make(map[string]string)
			}
			logic.Setting{}.Save(&entity.Setting{
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

	diskUsage := &accessor.DiskUsage{
		Usage: &types.DiskUsage{},
	}
	setting, err := logic.Setting{}.GetValue(logic.SettingGroupSetting, logic.SettingGroupSettingDiskUsage)
	if err == nil && setting != nil {
		diskUsage = setting.Value.DiskUsage
	}

	type portItem struct {
		Port accessor.PortItem `json:"port"`
		Name string            `json:"name"`
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
				imageInfo, _, err := docker.Sdk.Client.ImageInspectWithRaw(docker.Sdk.Ctx, item.ImageID)
				if err == nil {
					for port, _ := range imageInfo.Config.ExposedPorts {
						usePort = append(usePort, &portItem{
							Name: item.Names[0],
							Port: accessor.PortItem{
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
						Port: accessor.PortItem{
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
	statList, err := logic.Stat{}.GetStat()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"list": statList,
	})
	return
}

func (self Home) UpgradeScript(http *gin.Context) {
	containerRow, err := docker.Sdk.ContainerInfo(facade.GetConfig().GetString("app.name"))
	if err != nil {
		self.JsonResponseWithError(http, errors.New("您创建的面板容器名称非默认的 dpanel 无法获取更新脚本，请通过环境变量 APP_NAME 指定名称。"), 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"info": containerRow,
	})
	return
}
