package controller

import (
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/swarm"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"net"
	"strconv"
	"time"
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
	message := make([]string, 0)
	if info.Swarm.Error != "" {
		message = append(message, info.Swarm.Error)
	}
	if info.Swarm.Warnings != nil {
		message = append(message, info.Swarm.Warnings...)
	}
	result := gin.H{
		"status": gin.H{
			"isNode":   info.Swarm.LocalNodeState != swarm.LocalNodeStateInactive,
			"isManage": info.Swarm.ControlAvailable,
		},
		"info": info.Swarm,
	}
	if info.Swarm.ControlAvailable {
		swarmInfo, err := docker.Sdk.Client.SwarmInspect(docker.Sdk.Ctx)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		result["swarm"] = swarmInfo
		if info.Swarm.NodeID != "" {
			result["node"], _, err = docker.Sdk.Client.NodeInspectWithRaw(docker.Sdk.Ctx, info.Swarm.NodeID)
		}
	}
	self.JsonResponseWithoutError(http, result)
	return
}

func (self Swarm) Init(http *gin.Context) {
	type ParamsValidate struct {
		AdvertiseAddr    string `json:"advertiseAddr" binding:"ip|hostname|hostname_port"`
		ListenAddr       string `json:"listenAddr" binding:"ip|hostname|hostname_port"`
		DataPathAddr     string `json:"dataPathAddr" binding:"omitempty,ip|hostname|hostname_port"`
		ForceNewCluster  bool   `json:"forceNewCluster"`
		AutoLockManagers bool   `json:"autoLockManagers"`
		Subnet           string `json:"subnet"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	initRequest := swarm.InitRequest{
		ForceNewCluster:  false,
		AutoLockManagers: params.AutoLockManagers,
	}

	if addr, port, err := net.SplitHostPort(params.AdvertiseAddr); err == nil {
		initRequest.AdvertiseAddr = fmt.Sprintf("%s:%s", addr, port)
	} else {
		initRequest.AdvertiseAddr = fmt.Sprintf("%s:2377", params.AdvertiseAddr)
	}

	if addr, port, err := net.SplitHostPort(params.ListenAddr); err == nil {
		initRequest.ListenAddr = fmt.Sprintf("%s:%s", addr, port)
	} else {
		initRequest.ListenAddr = fmt.Sprintf("%s:2377", params.ListenAddr)
	}

	if params.DataPathAddr != "" {
		if addr, port, err := net.SplitHostPort(params.DataPathAddr); err == nil {
			initRequest.DataPathAddr = addr
			p, _ := strconv.Atoi(port)
			initRequest.DataPathPort = uint32(p)
		} else {
			initRequest.DataPathAddr = params.DataPathAddr
			initRequest.DataPathPort = 4789
		}
	}

	_, ipNet, err := net.ParseCIDR(params.Subnet)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	initRequest.DefaultAddrPool = []string{
		ipNet.String(),
	}
	subnetSize, _ := ipNet.Mask.Size()
	initRequest.SubnetSize = uint32(subnetSize)

	id, err := docker.Sdk.Client.SwarmInit(docker.Sdk.Ctx, initRequest)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	self.JsonResponseWithoutError(http, gin.H{
		"id": id,
	})
	return
}

func (self Swarm) NodeList(http *gin.Context) {
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

func (self Swarm) NodeJoin(http *gin.Context) {
	type ParamsValidate struct {
		DockerEnvName string         `json:"dockerEnvName" binding:"required"`
		Type          string         `json:"type" binding:"required,oneof=add join"`
		Role          swarm.NodeRole `json:"role" binding:"omitempty,oneof=worker manager"`
		ListenAddr    string         `json:"listenAddr" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	dockerEnv, err := logic.DockerEnv{}.GetEnvByName(params.DockerEnvName)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	dockerClient, err := docker.NewBuilderWithDockerEnv(dockerEnv)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		dockerClient.Close()
	}()

	var swarmDockerClient *docker.Builder
	var clientDockerClient *docker.Builder

	if params.Type == "join" {
		// join 是将当前环境添加到目标集群节点
		swarmDockerClient = dockerClient
		clientDockerClient = docker.Sdk
	} else {
		swarmDockerClient = docker.Sdk
		clientDockerClient = dockerClient
	}
	swarmDockerInfo, err := swarmDockerClient.Client.Info(swarmDockerClient.Ctx)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if swarmDockerInfo.Swarm.LocalNodeState == swarm.LocalNodeStateInactive {
		self.JsonResponseWithError(http, function.ErrorMessage(".swarmNotInit"), 500)
		return
	}
	if !swarmDockerInfo.Swarm.ControlAvailable {
		self.JsonResponseWithError(http, function.ErrorMessage(".swarmNotManager"), 500)
		return
	}
	swarmInfo, err := swarmDockerClient.Client.SwarmInspect(swarmDockerClient.Ctx)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	swarmManageNode, _, err := swarmDockerClient.Client.NodeInspectWithRaw(swarmDockerClient.Ctx, swarmDockerInfo.Swarm.NodeID)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	joinRequest := swarm.JoinRequest{
		ListenAddr:    params.ListenAddr,
		AdvertiseAddr: swarmManageNode.ManagerStatus.Addr,
		Availability:  swarm.NodeAvailabilityActive,
		RemoteAddrs: function.PluckArrayWalk(swarmDockerInfo.Swarm.RemoteManagers, func(item swarm.Peer) (string, bool) {
			return item.Addr, true
		}),
	}
	if ip, _, err := net.SplitHostPort(joinRequest.AdvertiseAddr); err == nil {
		joinRequest.DataPathAddr = ip
	}

	if params.Role == swarm.NodeRoleManager {
		joinRequest.JoinToken = swarmInfo.JoinTokens.Manager
	} else {
		joinRequest.JoinToken = swarmInfo.JoinTokens.Worker
	}

	err = clientDockerClient.Client.SwarmJoin(clientDockerClient.Ctx, joinRequest)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	leave := func(nodeId string) {
		_ = clientDockerClient.Client.SwarmLeave(clientDockerClient.Ctx, true)
		if nodeId != "" {
			_ = swarmDockerClient.Client.NodeRemove(swarmDockerClient.Ctx, nodeId, types.NodeRemoveOptions{})
		}
	}

	clientDockerInfo, err := clientDockerClient.Client.Info(clientDockerClient.Ctx)
	if err != nil {
		leave("")
		self.JsonResponseWithError(http, err, 500)
		return
	}

	for i := 0; i < 5; i++ {
		if _, err := clientDockerClient.Client.NetworkInspect(clientDockerClient.Ctx, "ingress", network.InspectOptions{
			Scope: "swarm",
		}); err == nil {
			self.JsonResponseWithoutError(http, gin.H{
				"id": clientDockerInfo.Swarm.NodeID,
			})
			return
		}
		time.Sleep(time.Second)
	}

	leave(clientDockerInfo.Swarm.NodeID)

	self.JsonResponseWithoutError(http, gin.H{
		"id":  "",
		"cmd": fmt.Sprintf("docker swarm join --token %s %s", joinRequest.JoinToken, joinRequest.AdvertiseAddr),
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
		err := docker.Sdk.Client.NodeRemove(docker.Sdk.Ctx, params.NodeId, types.NodeRemoveOptions{
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
	if nodeList, err := docker.Sdk.Client.NodeList(docker.Sdk.Ctx, types.NodeListOptions{}); err == nil {
		for _, node := range nodeList {
			if node.Status.State == swarm.NodeStateDown {
				_ = docker.Sdk.Client.NodeRemove(docker.Sdk.Ctx, node.ID, types.NodeRemoveOptions{})
			}
		}
	}
	self.JsonSuccessResponse(http)
	return
}
