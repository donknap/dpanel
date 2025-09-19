package controller

import (
	"bytes"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/swarm"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"io"
	"log/slog"
	"net"
	"strconv"
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

func (self Swarm) InfoJoin(http *gin.Context) {
	type ParamsValidate struct {
		DockerEnvName string         `json:"dockerEnvName" binding:"required"`
		Type          string         `json:"type" binding:"required,oneof=add join"`
		Role          swarm.NodeRole `json:"role" binding:"omitempty,oneof=worker manager"`
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
	if params.Type == "join" {
		// join 是将当前环境添加到目标集群节点
		swarmDockerClient = dockerClient
	} else {
		swarmDockerClient = docker.Sdk
	}
	swarmDockerInfo, err := swarmDockerClient.Client.Info(swarmDockerClient.Ctx)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if swarmDockerInfo.Swarm.LocalNodeState == swarm.LocalNodeStateInactive {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageSwarmNotInit), 500)
		return
	}
	if !swarmDockerInfo.Swarm.ControlAvailable {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageSwarmNotManager), 500)
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
	self.JsonResponseWithoutError(http, gin.H{
		"swarm": swarmInfo,
		"node":  swarmManageNode,
	})
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

func (self Swarm) Log(http *gin.Context) {
	type ParamsValidate struct {
		Id        string `json:"id" binding:"required"`
		Type      string `json:"type" binding:"required,oneof=service node"`
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
	progress, err := ws.NewFdProgressPip(http, fmt.Sprintf(ws.MessageTypeSwarmLog, params.Type, params.Id))
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	if progress.IsShadow() {
		option.Follow = false
	}
	var response io.ReadCloser
	if params.Type == "service" {
		response, err = docker.Sdk.Client.ServiceLogs(docker.Sdk.Ctx, params.Id, option)
	} else {
		response, err = docker.Sdk.Client.TaskLogs(docker.Sdk.Ctx, params.Id, option)
	}
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if params.Download {
		buffer, err := function.CombinedStdout(response)
		_ = response.Close()
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		http.Header("Content-Type", "text/plain")
		http.Header("Content-Disposition", "attachment; filename="+params.Id+".log")
		http.Data(200, "text/plain", buffer.Bytes())
		return
	}

	progress.OnWrite = func(p string) error {
		newReader := bytes.NewReader([]byte(p))
		stdout, err := function.CombinedStdout(newReader)
		if err != nil {
			progress.BroadcastMessage(p)
		} else {
			progress.BroadcastMessage(stdout.String())
		}
		return nil
	}

	go func() {
		if progress.IsShadow() {
			return
		}
		select {
		case <-progress.Done():
			slog.Debug("service run log response close", "err", fmt.Sprintf(ws.MessageTypeSwarmLog, params.Type, params.Id))
			_ = response.Close()
		}
	}()
	_, err = io.Copy(progress, response)
	self.JsonResponseWithoutError(http, gin.H{
		"log": "",
	})
}
