package controller

import (
	"encoding/json"
	"errors"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/we7coreteam/w7-rangine-go-support/src/facade"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
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

	client, err := logic.NewClientConn(http, &logic.ClientOptions{})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	go client.ReadMessage()
	go client.SendMessage()
}

func (self Home) WsConsole(http *gin.Context) {
	if !websocket.IsWebSocketUpgrade(http.Request) {
		self.JsonResponseWithError(http, errors.New("please connect using websocket"), 500)
		return
	}
	type ParamsValidate struct {
		Id      string `uri:"id" binding:"required"`
		Cmd     string `form:"cmd,default=/bin/sh111"`
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
	client, err := logic.NewClientConn(http, &logic.ClientOptions{
		CloseHandler: func() {
			shell.Close()
		},
		MessageHandler: map[string]func(message []byte){
			"console": func(message []byte) {
				var cmd command
				json.Unmarshal(message, &cmd)
				shell.Conn.Write([]byte(cmd.Content.Command))
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
			client.Conn.WriteMessage(websocket.TextMessage, []byte(processedOutput))
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
	dataUsage, err := docker.Sdk.Client.DiskUsage(docker.Sdk.Ctx, types.DiskUsageOptions{
		Types: []types.DiskUsageObject{
			types.ContainerObject,
			types.ImageObject,
			types.VolumeObject,
			types.BuildCacheObject,
		},
	})
	networkRow, _ := docker.Sdk.Client.NetworkList(docker.Sdk.Ctx, network.ListOptions{})
	containerTask, _ := dao.Site.Where(dao.Site.DeletedAt.IsNull()).Count()
	imageTask, _ := dao.Image.Count()
	self.JsonResponseWithoutError(http, gin.H{
		"info":       info,
		"usage":      dataUsage,
		"sdkVersion": docker.Sdk.Client.ClientVersion(),
		"total": map[string]int{
			"network":       len(networkRow),
			"containerTask": int(containerTask),
			"imageTask":     int(imageTask),
		},
		"dpanel": map[string]interface{}{
			"version":       facade.GetConfig().GetString("app.version"),
			"release":       "",
			"containerInfo": dpanelContainerInfo,
		},
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
