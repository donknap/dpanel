package controller

import (
	"github.com/docker/docker/api/types"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
)

func (self Swarm) ServiceList(http *gin.Context) {
	type ParamsValidate struct {
		NodeName string `json:"nodeName"`
	}
	list, err := docker.Sdk.Client.ServiceList(docker.Sdk.Ctx, types.ServiceListOptions{
		Status: true,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"list": list,
	})
	return
}
