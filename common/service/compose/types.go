package compose

import (
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/docker/api/types/container"
	"time"
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
	Health     string
}

type ContainerPublishersResult struct {
	URL           string `json:"url"`
	TargetPort    uint16 `json:"targetPort"`
	PublishedPort uint16 `json:"publishedPort"`
	Protocol      string `json:"protocol"`
}

type ProjectResult struct {
	// Name 任务的原始名称，为了兼容之前 dpanel-c 前缀的问题
	// 此名称是去掉前缀后的名称，与数据库中的任务名称对应
	Name string `json:"name"`
	// RunName 为 compose 实际运行的名称，可能会包含 dpanel-c 只在部署时候使用
	// 查询时均采用 Name
	RunName        string `json:"runName"`
	Status         string `json:"status"`
	ConfigFiles    string `json:"configFiles"`
	ConfigFileList []string
	IsDPanel       bool
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
