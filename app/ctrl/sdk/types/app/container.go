package app

import (
	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/common/accessor"
)

type ContainerDetailResult struct {
	Info   container.InspectResponse   `json:"info"`
	Ignore accessor.IgnoreCheckUpgrade `json:"ignore"`
}

type ContainerUpgradeOption struct {
	Md5       string `json:"md5" binding:"required"`
	ImageTag  string `json:"imageTag"`
	EnableBak bool   `json:"enableBak"`
}

type ContainerUpgradeResult struct {
	ContainerId string `json:"containerId"`
}
