package accessor

import (
	"github.com/donknap/dpanel/common/service/docker"
)

type SiteEnvOption struct {
	Name            string                    `json:"name"`
	Hostname        string                    `json:"hostname,omitempty"`
	ContainerName   string                    `json:"containerName,omitempty"`
	Environment     []docker.EnvItem          `json:"environment,omitempty"`
	Links           []docker.LinkItem         `json:"links,omitempty"`
	Ports           []docker.PortItem         `json:"ports,omitempty"`
	Volumes         []docker.VolumeItem       `json:"volumes,omitempty"`
	Network         []docker.NetworkItem      `json:"network,omitempty"`
	ImageName       string                    `json:"imageName,omitempty"` // 非表单提交
	ImageId         string                    `json:"imageId,omitempty"`   // 非表单提交
	Privileged      bool                      `json:"privileged,omitempty"`
	AutoRemove      bool                      `json:"autoRemove,omitempty"`
	Restart         string                    `json:"restart,omitempty"`
	Cpus            float32                   `json:"cpus,omitempty"`
	Memory          int                       `json:"memory,omitempty"`
	ShmSize         string                    `json:"shmsize,omitempty"`
	WorkDir         string                    `json:"workDir,omitempty"`
	User            string                    `json:"user,omitempty"`
	Command         string                    `json:"command,omitempty"`
	Entrypoint      string                    `json:"entrypoint,omitempty"`
	UseHostNetwork  bool                      `json:"useHostNetwork,omitempty"`
	BindIpV6        bool                      `json:"bindIpV6,omitempty"`
	Log             *docker.LogDriverItem     `json:"log,omitempty"`
	Dns             []string                  `json:"dns,omitempty"`
	Label           []docker.ValueItem        `json:"label,omitempty"`
	PublishAllPorts bool                      `json:"publishAllPorts,omitempty"`
	ExtraHosts      []docker.ValueItem        `json:"extraHosts,omitempty"`
	IpV4            *docker.NetworkCreateItem `json:"ipV4,omitempty"`
	IpV6            *docker.NetworkCreateItem `json:"ipV6,omitempty"`
	Device          []docker.DeviceItem       `json:"device,omitempty"`
	Gpus            *docker.GpusItem          `json:"gpus,omitempty"`
	Hook            *docker.HookItem          `json:"hook,omitempty"`
	Healthcheck     *docker.HealthcheckItem   `json:"healthcheck,omitempty"`
	HostPid         bool                      `json:"hostPid,omitempty"`
	CapAdd          []string                  `json:"capAdd,omitempty"`
}
