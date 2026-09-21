package controller

import (
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	agentTypes "github.com/donknap/dpanel/app/agent/types"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/gin-gonic/gin"
)

const (
	containerPortSourceHost     = "host"
	containerPortSourceDetected = "detected"
	containerPortSourceMapping  = "mapping"
)

type ContainerPortItem struct {
	container.Port
	Source    string `json:"Source,omitempty"`
	Listening bool   `json:"Listening"`
}

type containerPortFindResult struct {
	ContainerID string              `json:"containerId"`
	Ports       []ContainerPortItem `json:"ports"`
}

type containerPortCheckResult struct {
	ContainerID string                         `json:"containerId"`
	Ports       []logic.ContainerPortCheckItem `json:"ports"`
}

func (self Container) FindPort(http *gin.Context) {
	type ParamsValidate struct {
		ContainerIDs    []string `json:"containerIds"`
		Force           bool     `json:"force"`
		HostNetworkOnly bool     `json:"hostNetworkOnly"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	sdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if params.Force {
		if err = (logic.ContainerPort{}).Find(http.Request.Context(), sdk, params.ContainerIDs, params.HostNetworkOnly); err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	list := make([]containerPortFindResult, 0, len(params.ContainerIDs))
	for _, id := range params.ContainerIDs {
		info, err := sdk.Client.ContainerInspect(http.Request.Context(), id)
		if err != nil {
			self.JsonResponseWithError(http, fmt.Errorf("inspect container %s: %w", id, err), 500)
			return
		}
		cached, _ := storage.LoadCache[agentTypes.PortResult](
			fmt.Sprintf(storage.CacheKeyDockerContainerPort, sdk.Name, id),
		)
		ports := make([]ContainerPortItem, 0, len(cached.Ports))
		hostNetwork := info.HostConfig != nil && info.HostConfig.NetworkMode == network.NetworkHost
		for _, item := range cached.Ports {
			port := ContainerPortItem{
				Port: container.Port{
					PrivatePort: item.Port,
					Type:        item.Protocol,
				},
				Source:    containerPortSourceDetected,
				Listening: true,
			}
			if hostNetwork {
				port.IP = "0.0.0.0"
				port.PublicPort = item.Port
				port.Source = containerPortSourceHost
			}
			ports = append(ports, port)
		}
		list = append(list, containerPortFindResult{ContainerID: id, Ports: ports})
	}
	self.JsonResponseWithoutError(http, gin.H{"list": list})
}

func (self Container) CheckPort(http *gin.Context) {
	type ParamsValidate struct {
		ContainerID string   `json:"containerId" binding:"required"`
		Ports       []string `json:"ports"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	ports := make([]logic.ContainerPortCheckItem, 0, len(params.Ports))
	for _, address := range params.Ports {
		ports = append(ports, logic.ContainerPortCheckItem{Address: address})
	}
	ports = function.UniqueArrayWalk(ports, func(item logic.ContainerPortCheckItem) string {
		return item.Address
	})
	if err := (logic.ContainerPort{}).Check(http.Request.Context(), &ports); err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, containerPortCheckResult{
		ContainerID: params.ContainerID,
		Ports:       ports,
	})
}
