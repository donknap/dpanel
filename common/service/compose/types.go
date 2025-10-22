package compose

import (
	"time"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/docker/api/types/container"
)

type Service struct {
	types.ServiceConfig
	XDPanelService ExtService `yaml:"x-dpanel-service,omitempty" json:"x-dpanel-service,omitempty"`
}

type ContainerResult struct {
	Name       string                      `json:"name"`
	Project    string                      `json:"project"`
	Service    string                      `json:"service"`
	Publishers []ContainerPublishersResult `json:"publishers"`
	State      string                      `json:"state"`
	Status     string                      `json:"status"`
	Health     string                      `json:"health"`
}

type ContainerPublishersResult struct {
	URL           string `json:"url"`
	TargetPort    uint16 `json:"targetPort"`
	PublishedPort uint16 `json:"publishedPort"`
	Protocol      string `json:"protocol"`
}

type ProjectResult struct {
	Name string
	// 当前任务实际运行的名称，使终保持 Name == RunName
	// 旧版带前缀的名称会放置到 RunName 中，销毁重建后则会恢复到原始名称
	RunName        string // Deprecated
	Status         string
	ConfigFiles    string
	ConfigFileList []string
	UpdatedAt      time.Time
	ContainerList  []TaskResultRunContainerResult
	CanManage      bool
	Workdir        string
}

type TaskResultRunContainerResult struct {
	Container  container.Summary
	ConfigHash string
	Service    string
}
