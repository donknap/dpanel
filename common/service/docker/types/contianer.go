package types

import (
	"strings"

	"github.com/docker/docker/api/types/filters"
)

// 容器相关
type VolumeItem struct {
	Host       string `json:"host"`
	Dest       string `json:"dest"`
	Permission string `json:"permission"` // readonly or write
	InImage    bool   `json:"inImage,omitempty"`
	Type       string `json:"type,omitempty"`
}

type LinkItem struct {
	Name   string `json:"name"`
	Alise  string `json:"alise"`
	Volume bool   `json:"volume"`
}

type NetworkItem struct {
	Name       string   `json:"name"`
	Alise      []string `json:"alise"`
	IpV4       string   `json:"ipV4"`
	IpV6       string   `json:"ipV6"`
	MacAddress string   `json:"macAddress"`
	DnsName    []string `json:"dnsName"`
}

type ValueItem struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type DeviceItem struct {
	Host string `json:"host"`
	Dest string `json:"dest"`
}

type PortItem struct {
	HostIp   string `json:"hostIp"`
	Host     string `json:"host"`
	Dest     string `json:"dest"`
	Protocol string `json:"protocol"`
	Mode     string `json:"mode"`
}

func (self *PortItem) Parse() PortItem {
	if hostIp, port, exists := strings.Cut(self.Host, ":"); exists {
		self.HostIp = hostIp
		self.Host = port
	}
	if port, protocol, exists := strings.Cut(self.Dest, "/"); exists {
		self.Dest = port
		self.Protocol = protocol
	}
	return *self
}

type LogDriverItem struct {
	Driver  string `json:"driver"`
	MaxFile string `json:"maxFile"`
	MaxSize string `json:"maxSize"`
}

type GpusItem struct {
	Enable       bool     `json:"enable"`
	Device       []string `json:"device"`
	Capabilities []string `json:"capabilities"`
}

type HookItem struct {
	ContainerStart  string `json:"containerStart"`
	ContainerCreate string `json:"containerCreate"`
}

type HealthcheckItem struct {
	ShellType string `json:"shellType"`
	Cmd       string `json:"cmd"`
	Interval  int    `json:"interval"`
	Timeout   int    `json:"timeout"`
	Retries   int    `json:"retries"`
}

type NetworkCreateItem struct {
	Address string `json:"address"`
	Subnet  string `json:"subnet"`
	Gateway string `json:"gateway"`
}

// Deprecated: instead PlatformArch
type ImagePlatform struct {
	Type string `json:"type"`
	Arch string `json:"arch"`
}

type FileItemResult struct {
	ShowName string `json:"showName"` // 展示名称，包含名称 + link 名称
	Name     string `json:"name"`     // 完整的路径名称，不包含 linkname，eg: /dpanel/compose/compose1
	LinkName string `json:"linkName"` // 链接目录或是文件
	Size     string `json:"size"`
	Mode     string `json:"mode"`
	IsDir    bool   `json:"isDir"`
	ModTime  string `json:"modTime"`
	Change   int    `json:"change"`
	Group    string `json:"group"`
	Owner    string `json:"owner"`
}

type RestartPolicy struct {
	Name       string `json:"name" binding:"omitempty,oneof=no on-failure unless-stopped always any none"`
	MaxAttempt int    `json:"maxAttempt"`
	Delay      int    `json:"delay"`
	Window     int    `json:"window"`
}

type Constraint struct {
	Role  string `json:"role"`
	Node  string `json:"node"`
	Label []struct {
		Name     string `json:"name"`
		Value    string `json:"value"`
		Operator string `json:"operator"`
	} `json:"label"`
}

type Scheduling struct {
	Mode     string `json:"mode"`
	Replicas int    `json:"replicas,omitempty"`
	Update   struct {
		Delay         int    `json:"delay"`
		Parallelism   int    `json:"parallelism,omitempty"`
		FailureAction string `json:"failureAction"`
		Order         string `json:"order"`
	} `json:"update"`
}

type ContainerStatsOption struct {
	Stream  bool
	Filters filters.Args
}
