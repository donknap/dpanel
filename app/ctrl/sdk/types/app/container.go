package app

import (
	"github.com/docker/docker/api/types/container"
)

type ContainerDetailResult struct {
	Info container.InspectResponse `json:"info"`
}

type ContainerUpgradeOption struct {
	Md5       string `json:"md5" binding:"required"`
	ImageTag  string `json:"imageTag"`
	EnableBak bool   `json:"enableBak"`
}

type ContainerUpgradeResult struct {
	ContainerId string `json:"containerId"`
}

type ContainerCheckUpgradeOption struct {
	ContainerID string `json:"containerId"`
	Force       bool   `json:"force"`
}

type ContainerCheckUpgradeResult struct {
	Upgrade     bool     `json:"upgrade"`
	Digest      string   `json:"digest"`
	DigestLocal []string `json:"digestLocal"`
	Error       string   `json:"error"`
	Status      string   `json:"status"`
}

type ContainerBackupOption struct {
	Id               string   `json:"id"`
	BackupImage      string   `json:"backupImage"`
	BackupVolume     string   `json:"backupVolume"`
	BackupVolumeList []string `json:"backupVolumeList"`
}

type ContainerBackupResult struct {
	Path string `json:"path"`
}
