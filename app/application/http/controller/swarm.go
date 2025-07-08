package controller

import (
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"net"
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
		AdvertiseAddr    string `json:"advertiseAddr" binding:"required"`
		ListenAddr       string `json:"listenAddr"`
		ForceNewCluster  bool   `json:"forceNewCluster"`
		AutoLockManagers bool   `json:"autoLockManagers"`
		Port             int    `json:"port"`
		Subnet           string `json:"subnet"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if params.Port == 0 {
		params.Port = 2377
	}
	if params.ListenAddr == "" {
		params.ListenAddr = "0.0.0.0"
	}
	_, ipNet, err := net.ParseCIDR(params.Subnet)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	var subnet string
	var subnetSize int

	subnetSize, _ = ipNet.Mask.Size()
	subnet = ipNet.String()

	id, err := docker.Sdk.Client.SwarmInit(docker.Sdk.Ctx, swarm.InitRequest{
		ListenAddr:       fmt.Sprintf("%s:%d", params.ListenAddr, params.Port),
		AdvertiseAddr:    fmt.Sprintf("%s:%d", params.AdvertiseAddr, params.Port),
		ForceNewCluster:  false,
		AutoLockManagers: params.AutoLockManagers,
		DefaultAddrPool: []string{
			subnet,
		},
		SubnetSize: uint32(subnetSize),
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
	options := []docker.Option{
		docker.WithName(dockerEnv.Name),
		docker.WithAddress(dockerEnv.Address),
	}
	if dockerEnv.EnableTLS {
		options = append(options, docker.WithTLS(dockerEnv.TlsCa, dockerEnv.TlsCert, dockerEnv.TlsKey))
	}
	dockerClient, err := docker.NewBuilder(options...)
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
	//subnet := make([]network.IPAMConfig, 0)
	//if !function.IsEmptyArray(swarmInfo.DefaultAddrPool) {
	//	subnet = function.PluckArrayWalk(swarmInfo.DefaultAddrPool, func(value string) (network.IPAMConfig, bool) {
	//		ip, _, _ := net.ParseCIDR(value)
	//		return network.IPAMConfig{
	//			Subnet:  value,
	//			Gateway: fmt.Sprintf("%d.%d.%d.1", ip.To4()[0], ip.To4()[1], ip.To4()[2]),
	//		}, true
	//	})
	//} else {
	//	subnet = append(subnet, network.IPAMConfig{
	//		Subnet:  fmt.Sprintf("10.0.0.1/%d", swarmInfo.SubnetSize),
	//		Gateway: "10.0.0.1",
	//	})
	//}
	//_, err = clientDockerClient.Client.NetworkCreate(dockerClient.Ctx, "ingress", network.CreateOptions{
	//	Driver:     "overlay",
	//	Scope:      "swarm",
	//	EnableIPv4: function.PtrBool(true),
	//	IPAM: &network.IPAM{
	//		Driver: "default",
	//		Config: subnet,
	//	},
	//	Ingress: true,
	//})
	//if err != nil {
	//	self.JsonResponseWithError(http, err, 500)
	//	return
	//}
	self.JsonSuccessResponse(http)
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
