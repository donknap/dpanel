package controller

import (
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	agentTypes "github.com/donknap/dpanel/app/agent/types"
	"github.com/donknap/dpanel/app/application/logic"
	serviceAgent "github.com/donknap/dpanel/common/service/agent"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
)

func (self Container) CheckPort(http *gin.Context) {
	type ParamsValidate struct {
		ContainerIDs []string `json:"containerId"`
		EnableForce  bool     `json:"enableForce"`
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
	containerFilters := filters.NewArgs()
	for _, containerID := range params.ContainerIDs {
		containerFilters.Add("id", containerID)
	}
	options := container.ListOptions{All: true}
	if len(params.ContainerIDs) > 0 {
		options.Filters = containerFilters
	}
	containers, err := sdk.ContainerList(http.Request.Context(), options)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	containerPort := logic.ContainerPort{}
	targets := make([]serviceAgent.PortCheckTarget, 0, len(containers))
	for _, item := range containers {
		info, err := sdk.Client.ContainerInspect(http.Request.Context(), item.ID)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		ports, checkable := containerPort.CheckablePorts(info)
		if !checkable {
			continue
		}
		targets = append(targets, serviceAgent.PortCheckTarget{
			ContainerID: info.ID,
			Ports:       ports,
		})
	}
	var result []agentTypes.PortCheckResult
	if params.EnableForce {
		result, err = containerPort.Check(http.Request.Context(), sdk, targets)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	} else {
		result = containerPort.LoadCache(sdk.Name, targets)
	}
	total, success, failed, timeout := 0, 0, 0, 0
	for _, containerResult := range result {
		for _, port := range containerResult.Ports {
			if port.Port == "0" {
				continue
			}
			total++
			switch port.Status {
			case "success":
				success++
			case "failed":
				failed++
			case "timeout":
				timeout++
			}
		}
	}
	self.JsonResponseWithoutError(http, gin.H{
		"failed":  failed,
		"list":    result,
		"success": success,
		"timeout": timeout,
		"total":   total,
	})
}
