package types

import (
	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/common/service/docker/types"
)

const (
	DPanelRunInContainer = "container" // 在容器中运行
	DPanelRunInHost      = "host"      // 在宿主机运行
)

type DPanelInfo struct {
	ContainerInfo container.InspectResponse `json:"containerInfo"`
	Name          string                    `json:"name"`
	Version       string                    `json:"version"`
	Family        string                    `json:"family"`
	Env           string                    `json:"env"`
	RunIn         string                    `json:"runIn"`

	BaseURL          string `json:"baseUrl"`
	BaseImage        string `json:"baseImage,omitempty"`
	ServerHost       string `json:"serverHost"`
	ServerPort       int    `json:"serverPort"`
	PublicPort       int    `json:"publicPort,omitempty"`
	LogFileLevel     string `json:"logFileLevel"`
	LogConsoleLevel  string `json:"logConsoleLevel"`
	StorageLocalPath string `json:"storageLocalPath"`

	Dns     string `json:"dns"`
	Proxy   string `json:"proxy"`
	NoProxy string `json:"noProxy"`

	// 首项为 /dpanel；容器运行时后续项为独立子挂载，二进制运行时仅一项。
	DataMounts []types.VolumeItem `json:"dataMounts"`

	IsDev  bool `json:"isDev"`
	IsCe   bool `json:"isCe"`
	IsLite bool `json:"isLite"`
}
