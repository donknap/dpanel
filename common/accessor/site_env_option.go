package accessor

import (
	"strings"
)

type VolumeItem struct {
	Host       string `json:"host"`
	Dest       string `json:"dest"`
	Permission string `json:"permission"`
	InImage    bool   `json:"inImage"`
}

type LinkItem struct {
	Name   string `json:"name"`
	Alise  string `json:"alise"`
	Volume bool   `json:"volume"`
}

type NetworkItem struct {
	Name  string   `json:"name"`
	Alise []string `json:"alise"`
	IpV4  string   `json:"ipV4"`
	IpV6  string   `json:"ipV6"`
}

type EnvItem struct {
	Label   string `json:"label,omitempty" yaml:"label,omitempty"`
	Default string `json:"default,omitempty" yaml:"default,omitempty"`
	Name    string `json:"name"`
	Value   string `json:"value"`
}

type PortItem struct {
	HostIp   string `json:"hostIp"`
	Host     string `json:"host"`
	Dest     string `json:"dest"`
	Protocol string `json:"protocol"`
}

func (self PortItem) Parse() PortItem {
	newValue := PortItem{
		HostIp:   self.HostIp,
		Host:     self.Host,
		Dest:     self.Dest,
		Protocol: self.Protocol,
	}

	if strings.Contains(self.Host, ":") {
		temp := strings.Split(self.Host, ":")
		newValue.HostIp = temp[0]
		newValue.Host = temp[1]
	}
	if newValue.Protocol == "" {
		newValue.Protocol = "tcp"
	}
	return newValue
}

type LogDriverItem struct {
	Driver  string `json:"driver"`
	MaxFile string `json:"maxFile"`
	MaxSize string `json:"maxSize"`
}

type ContainerNetworkItem struct {
	Address string `json:"address"`
	Subnet  string `json:"subnet"`
	Gateway string `json:"gateway"`
}

type ReplaceItem struct {
	Depend string `json:"depend"`
	Target string `json:"target"`
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

type SiteEnvOption struct {
	Name            string                `json:"name"`
	ContainerName   string                `json:"containerName,omitempty"`
	Environment     []EnvItem             `json:"environment,omitempty"`
	Links           []LinkItem            `json:"links,omitempty"`
	Ports           []PortItem            `json:"ports,omitempty"`
	Volumes         []VolumeItem          `json:"volumes,omitempty"`
	VolumesDefault  []VolumeItem          `json:"volumesDefault,omitempty"`
	Network         []NetworkItem         `json:"network,omitempty"`
	ImageName       string                `json:"imageName,omitempty"` // 非表单提交
	ImageId         string                `json:"imageId,omitempty"`   // 非表单提交
	Privileged      bool                  `json:"privileged,omitempty"`
	AutoRemove      bool                  `json:"autoRemove,omitempty"`
	Restart         string                `json:"restart,omitempty"`
	Cpus            float32               `json:"cpus,omitempty"`
	Memory          int                   `json:"memory,omitempty"`
	ShmSize         string                `json:"shmsize,omitempty"`
	WorkDir         string                `json:"workDir,omitempty"`
	User            string                `json:"user,omitempty"`
	Command         string                `json:"command,omitempty"`
	Entrypoint      string                `json:"entrypoint,omitempty"`
	UseHostNetwork  bool                  `json:"useHostNetwork,omitempty"`
	BindIpV6        bool                  `json:"bindIpV6,omitempty"`
	Log             *LogDriverItem        `json:"log,omitempty"`
	Dns             []string              `json:"dns,omitempty"`
	Label           []EnvItem             `json:"label,omitempty"`
	PublishAllPorts bool                  `json:"publishAllPorts,omitempty"`
	ExtraHosts      []EnvItem             `json:"extraHosts,omitempty"`
	IpV4            *ContainerNetworkItem `json:"ipV4,omitempty"`
	IpV6            *ContainerNetworkItem `json:"ipV6,omitempty"`
	Replace         []ReplaceItem         `json:"replace,omitempty"`
	Device          []VolumeItem          `json:"device,omitempty"`
	Gpus            *GpusItem             `json:"gpus,omitempty"`
	Hook            *HookItem             `json:"hook,omitempty"`
	Healthcheck     *HealthcheckItem      `json:"healthcheck,omitempty"`
	HostPid         bool                  `json:"hostPid,omitempty"`
}
