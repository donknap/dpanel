package controller

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type Swarm struct {
	controller.Abstract
}

func (self Swarm) Info(http *gin.Context) {
	info, err := docker.Sdk.Client.Info(docker.Sdk.Ctx)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	result := gin.H{
		"status": gin.H{
			"isNode":   info.Swarm.NodeID != "",
			"isManage": info.Swarm.ControlAvailable,
		},
	}
	if info.Swarm.ControlAvailable {
		swarmInfo, err := docker.Sdk.Client.SwarmInspect(docker.Sdk.Ctx)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		result["swarm"] = swarmInfo
	}
	if info.Swarm.NodeID != "" {
		result["info"], _, err = docker.Sdk.Client.NodeInspectWithRaw(docker.Sdk.Ctx, info.Swarm.NodeID)
	}
	self.JsonResponseWithoutError(http, result)
	return
}

func (self Swarm) Create(http *gin.Context) {
	type ParamsValidate struct {
		AdvertiseAddr    string `json:"advertiseAddr" binding:"required"`
		ListenAddr       string `json:"listenAddr"`
		ForceNewCluster  bool   `json:"forceNewCluster"`
		AutoLockManagers bool   `json:"autoLockManagers"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if params.ListenAddr == "" {
		params.ListenAddr = params.AdvertiseAddr
	}

	id, err := docker.Sdk.Client.SwarmInit(docker.Sdk.Ctx, swarm.InitRequest{
		ListenAddr:       params.ListenAddr,
		AdvertiseAddr:    params.AdvertiseAddr,
		ForceNewCluster:  false,
		AutoLockManagers: params.AutoLockManagers,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	self.JsonResponseWithoutError(http, gin.H{
		"id": id,
	})
	return
}

func (self Swarm) GetNodeList(http *gin.Context) {
	list, err := docker.Sdk.Client.NodeList(docker.Sdk.Ctx, types.NodeListOptions{})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"list": list,
	})
	return
}
