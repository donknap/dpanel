package types

import "github.com/docker/docker/api/types/container"

const (
	DPanelRunInContainer     = "container" // 在容器中运行
	DPanelRunInHost          = "host"      // 在宿主机运行
	DPanelRunInDockerDesktop = "dockerDesktop"
)

type DPanelInfo struct {
	ContainerInfo container.InspectResponse
	RunIn         string
	Proxy         string
}
