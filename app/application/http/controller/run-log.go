package controller

import (
	"fmt"
	"io"
	"log/slog"
	"strconv"

	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type RunLog struct {
	controller.Abstract
}

func (self RunLog) Run(http *gin.Context) {
	type ParamsValidate struct {
		Id        string `json:"id" binding:"required"`
		LineTotal int    `json:"lineTotal" binding:"required,number,oneof=50 100 200 500 1000 5000 -1"`
		Download  bool   `json:"download"`
		ShowTime  bool   `json:"showTime"`
	}

	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	option := container.LogsOptions{
		ShowStderr: true,
		ShowStdout: true,
		Follow:     !params.Download,
		Timestamps: params.ShowTime,
	}
	if params.LineTotal > 0 {
		option.Tail = strconv.Itoa(params.LineTotal)
	}
	progress, err := ws.NewFdProgressPip(http, fmt.Sprintf(ws.MessageTypeContainerLog, params.Id))
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	slog.Debug("container run log progress", "detail", progress.String())
	if progress.IsShadow() {
		option.Follow = false
	}

	if params.Download {
		response, err := docker.Sdk.ContainerLogs(docker.Sdk.Ctx, params.Id, option)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		defer func() {
			_ = response.Close()
		}()
		http.Header("Content-Type", "text/plain")
		http.Header("Content-Disposition", "attachment; filename="+params.Id+".log")
		http.DataFromReader(200, 0, "text/plain", response, nil)
		return
	}

	response, err := docker.Sdk.ContainerLogs(docker.Sdk.Ctx, params.Id, option)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	progress.OnWrite = func(p string) error {
		progress.BroadcastMessage(p)
		return nil
	}

	go func() {
		if progress.IsShadow() {
			return
		}
		select {
		case <-progress.Done():
			slog.Debug("container", "run log response close", fmt.Sprintf(ws.MessageTypeContainerLog, params.Id))
			_ = response.Close()
		}
	}()

	_, err = io.Copy(progress, response)
	if err != nil {
		slog.Debug("container run log copy", "err", err)
	}
	self.JsonResponseWithoutError(http, gin.H{
		"log": "",
	})
	return
}
