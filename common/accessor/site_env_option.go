package accessor

import (
	"github.com/donknap/dpanel/common/service/docker/types"
)

type SiteEnvOption struct {
	DockerEnvName   string                   `json:"dockerEnvName,omitempty"`
	Name            string                   `json:"name,omitempty"`
	Hostname        string                   `json:"hostname,omitempty"`
	ContainerName   string                   `json:"containerName,omitempty"`
	Environment     []types.EnvItem          `json:"environment,omitempty"`
	Links           []types.LinkItem         `json:"links,omitempty"`
	Ports           []types.PortItem         `json:"ports,omitempty"`
	Volumes         []types.VolumeItem       `json:"volumes,omitempty"`
	Network         []types.NetworkItem      `json:"network,omitempty"`
	ImageName       string                   `json:"imageName,omitempty"` // 非表单提交
	ImageId         string                   `json:"imageId,omitempty"`   // 非表单提交
	Privileged      bool                     `json:"privileged,omitempty"`
	AutoRemove      bool                     `json:"autoRemove,omitempty"`
	RestartPolicy   *types.RestartPolicy     `json:"restartPolicy,omitempty"`
	Cpus            float32                  `json:"cpus,omitempty"`
	Memory          int                      `json:"memory,omitempty"`
	ShmSize         string                   `json:"shmsize,omitempty"`
	WorkDir         string                   `json:"workDir,omitempty"`
	User            string                   `json:"user,omitempty"`
	Command         string                   `json:"command,omitempty"`
	Entrypoint      string                   `json:"entrypoint,omitempty"`
	UseHostNetwork  bool                     `json:"useHostNetwork,omitempty"`
	BindIpV6        bool                     `json:"bindIpV6,omitempty"`
	Log             *types.LogDriverItem     `json:"log,omitempty"`
	Dns             []string                 `json:"dns,omitempty"`
	Label           []types.ValueItem        `json:"label,omitempty"`
	PublishAllPorts bool                     `json:"publishAllPorts,omitempty"`
	ExtraHosts      []types.ValueItem        `json:"extraHosts,omitempty"`
	IpV4            *types.NetworkCreateItem `json:"ipV4,omitempty"`
	IpV6            *types.NetworkCreateItem `json:"ipV6,omitempty"`
	Device          []types.DeviceItem       `json:"device,omitempty"`
	Gpus            *types.GpusItem          `json:"gpus,omitempty"`
	Hook            *types.HookItem          `json:"hook,omitempty"`
	Healthcheck     *types.HealthcheckItem   `json:"healthcheck,omitempty"`
	HostPid         bool                     `json:"hostPid,omitempty"`
	CapAdd          []string                 `json:"capAdd,omitempty"`
	Constraint      *types.Constraint        `json:"constraint,omitempty"`
	ImageRegistry   int32                    `json:"imageRegistry,omitempty"`
	Placement       []types.ValueItem        `json:"placement,omitempty"`
	Scheduling      *types.Scheduling        `json:"scheduling,omitempty"`
	Restart         string                   `json:"restart,omitempty"` // Deprecated: instead RestartPolicy
	GroupAdd        []string                 `json:"groupAdd,omitempty"`
	Init            bool                     `json:"init,omitempty"`
}
