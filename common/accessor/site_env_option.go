package accessor

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
	Name  string `json:"name"`
	Value string `json:"value"`
}

type PortItem struct {
	HostIp string `json:"hostIp"`
	Host   string `json:"host"`
	Dest   string `json:"dest"`
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

type SiteEnvOption struct {
	Environment     []EnvItem            `json:"environment"`
	Links           []LinkItem           `json:"links"`
	Ports           []PortItem           `json:"ports"`
	Volumes         []VolumeItem         `json:"volumes"`
	VolumesDefault  []VolumeItem         `json:"volumesDefault"`
	Network         []NetworkItem        `json:"network"`
	ImageName       string               `json:"imageName"` // 非表单提交
	ImageId         string               `json:"imageId"`   // 非表单提交
	Privileged      bool                 `json:"privileged"`
	AutoRemove      bool                 `json:"autoRemove"`
	Restart         string               `json:"restart"`
	Cpus            float32              `json:"cpus"`
	Memory          int                  `json:"memory"`
	ShmSize         string               `json:"shmsize,omitempty"`
	WorkDir         string               `json:"workDir"`
	User            string               `json:"user"`
	Command         string               `json:"command"`
	Entrypoint      string               `json:"entrypoint"`
	UseHostNetwork  bool                 `json:"useHostNetwork"`
	BindIpV6        bool                 `json:"bindIpV6"`
	Log             LogDriverItem        `json:"log"`
	Dns             []string             `json:"dns"`
	Label           []EnvItem            `json:"label"`
	PublishAllPorts bool                 `json:"publishAllPorts"`
	ExtraHosts      []EnvItem            `json:"extraHosts"`
	IpV4            ContainerNetworkItem `json:"ipV4"`
	IpV6            ContainerNetworkItem `json:"ipV6"`
}
