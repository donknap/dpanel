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
	ServerHost       string `json:"serverHost"`
	ServerPort       int    `json:"serverPort"`
	LogFileLevel     string `json:"logFileLevel"`
	LogConsoleLevel  string `json:"logConsoleLevel"`
	StorageLocalPath string `json:"storageLocalPath"`

	Dns     string           `json:"dns"`
	Proxy   string           `json:"proxy"`
	NoProxy string           `json:"noProxy"`
	Mount   types.VolumeItem `json:"mount"` // 通过容器创建时是挂载目录，二进制运行时是直接路径

	IsDev  bool `json:"isDev"`
	IsCe   bool `json:"isCe"`
	IsLite bool `json:"isLite"`
}
