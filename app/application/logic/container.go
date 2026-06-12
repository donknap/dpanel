package logic

import (
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/common/service/docker"
)

type Container struct {
}

type ContainerRuntimeStatus struct {
	Unhealthy bool
	State     container.ContainerState
	Message   string
}

type ContainerRuntimeItem struct {
	Summary container.Summary
	Inspect *container.InspectResponse
}

func (self Container) RuntimeStatus(item ContainerRuntimeItem) ContainerRuntimeStatus {
	result := ContainerRuntimeStatus{}
	if strings.Contains(item.Summary.Status, "unhealthy") || strings.Contains(item.Summary.Status, "Restarting") {
		result.Unhealthy = true
	}
	if item.Inspect == nil || !self.runtimeRestarting(*item.Inspect) {
		return result
	}
	result.Unhealthy = true
	result.State = container.ContainerState(container.Unhealthy)
	result.Message = "Frequent restarts"
	return result
}

func (self Container) runtimeRestarting(inspectInfo container.InspectResponse) bool {
	if inspectInfo.State != nil && inspectInfo.State.Restarting && inspectInfo.RestartCount > 0 {
		return true
	}
	if inspectInfo.Config == nil || inspectInfo.Config.Healthcheck != nil {
		return false
	}
	if inspectInfo.RestartCount >= 3 {
		return true
	}
	runtime, ok := docker.Sdk.ContainerRuntime(docker.Sdk.Ctx, inspectInfo.ID)
	if !ok {
		return false
	}
	return runtime.ActionCount("start", "restart") >= 3
}
