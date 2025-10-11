package controller

import (
	"fmt"
	"io"
	"time"

	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/gin-gonic/gin"
)

func (self Container) GetStatInfo(http *gin.Context) {
	type ParamsValidate struct {
		Id string `json:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	progress, err := ws.NewFdProgressPip(http, fmt.Sprintf(ws.MessageTypeContainerStat, params.Id))
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	response, err := docker.Sdk.Client.ContainerStats(progress.Context(), params.Id, true)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	lastSendTime := time.Now()

	progress.OnWrite = func(p string) error {
		if time.Now().Sub(lastSendTime) < time.Second*2 {
			return nil
		}
		lastSendTime = time.Now()
		progress.BroadcastMessage(p)
		return nil
	}
	_, err = io.Copy(progress, response.Body)
	self.JsonSuccessResponse(http)
	return
}

func (self Container) GetProcessInfo(http *gin.Context) {
	type ParamsValidate struct {
		Id string `json:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	psInfo, err := docker.Sdk.Client.ContainerTop(docker.Sdk.Ctx, params.Id, nil)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"list": psInfo,
	})
	return
}
