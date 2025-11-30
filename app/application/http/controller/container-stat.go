package controller

import (
	"fmt"

	"github.com/docker/docker/api/types/filters"
	"github.com/donknap/dpanel/common/function"
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
	_, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, params.Id)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	progress, err := ws.NewFdProgressPip(http, fmt.Sprintf(ws.MessageTypeContainerStat, params.Id))
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	response, err := docker.Sdk.ContainerStats(progress.Context(), docker.ContainerStatsOption{
		Stream:  true,
		Filters: filters.NewArgs(filters.Arg("id", params.Id)),
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	for {
		select {
		case <-progress.Done():
			self.JsonSuccessResponse(http)
			return
		case list, ok := <-response:
			if !ok {
				// 关闭通道继续执行，正常回收资源
				progress.Close()
				continue
			}
			if !function.IsEmptyArray(list) {
				progress.BroadcastMessage(list[0])
			}
		}
	}
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
