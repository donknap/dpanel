package app

import (
	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/common/accessor"
)

type ContainerDetailResult struct {
	Info   container.InspectResponse            `json:"info"`
	Ignore accessor.ContainerCheckIgnoreUpgrade `json:"ignore"`
}

type ContainerUpgradeOption struct {
	Md5       string `json:"md5" binding:"required"`
	ImageTag  string `json:"imageTag"`
	EnableBak bool   `json:"enableBak"`
}

type ContainerUpgradeResult struct {
	ContainerId string `json:"containerId"`
}

type ContainerBackupOption struct {
	Id                string   `json:"id"`
	EnableImage       bool     `json:"enableImage"`
	EnableCommitImage bool     `json:"enableCommitImage"`
	Volume            []string `json:"volume"`
}

type ContainerBackupResult struct {
	Path string `json:"path"`
}
