package controller

import (
	"github.com/docker/docker/api/types/swarm"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
)

func (self Swarm) NodeList(http *gin.Context) {
	list, err := docker.Sdk.Client.NodeList(docker.Sdk.Ctx, swarm.NodeListOptions{})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"list": list,
	})
	return
}

func (self Swarm) NodeUpdate(http *gin.Context) {
	type ParamsValidate struct {
		NodeId       string                 `json:"nodeId" binding:"required"`
		Availability swarm.NodeAvailability `json:"availability" binding:"omitempty,oneof=active pause drain"`
		Role         swarm.NodeRole         `json:"role" binding:"omitempty,oneof=worker manager"`
		Labels       []docker.ValueItem     `json:"labels"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	node, _, err := docker.Sdk.Client.NodeInspectWithRaw(docker.Sdk.Ctx, params.NodeId)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if params.Availability != "" {
		node.Spec.Availability = params.Availability
	}
	if params.Role != "" {
		node.Spec.Role = params.Role
	}
	if !function.IsEmptyArray(params.Labels) {
		node.Spec.Labels = function.PluckArrayMapWalk(params.Labels, func(item docker.ValueItem) (string, string, bool) {
			return item.Name, item.Value, true
		})
	}
	err = docker.Sdk.Client.NodeUpdate(docker.Sdk.Ctx, params.NodeId, node.Version, node.Spec)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"detail": node,
	})
	return
}

func (self Swarm) NodeRemove(http *gin.Context) {
	type ParamsValidate struct {
		NodeId string `json:"nodeId"`
		Force  bool   `json:"force"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	info, err := docker.Sdk.Client.Info(docker.Sdk.Ctx)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if info.Swarm.ControlAvailable && params.NodeId != "" {
		err := docker.Sdk.Client.NodeRemove(docker.Sdk.Ctx, params.NodeId, swarm.NodeRemoveOptions{
			Force: params.Force,
		})
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	} else {
		err := docker.Sdk.Client.SwarmLeave(docker.Sdk.Ctx, params.Force)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Swarm) NodePrune(http *gin.Context) {
	if nodeList, err := docker.Sdk.Client.NodeList(docker.Sdk.Ctx, swarm.NodeListOptions{}); err == nil {
		for _, node := range nodeList {
			if node.Status.State == swarm.NodeStateDown {
				_ = docker.Sdk.Client.NodeRemove(docker.Sdk.Ctx, node.ID, swarm.NodeRemoveOptions{})
			}
		}
	}
	self.JsonSuccessResponse(http)
	return
}
