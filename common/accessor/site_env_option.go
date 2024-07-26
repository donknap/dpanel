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

type SiteEnvOption struct {
	Environment    []EnvItem     `json:"environment"`
	Links          []LinkItem    `json:"links"`
	Ports          []PortItem    `json:"ports"`
	Volumes        []VolumeItem  `json:"volumes"`
	VolumesDefault []VolumeItem  `json:"volumesDefault"`
	Network        []NetworkItem `json:"network"`
	ImageName      string        `json:"imageName"`
	ImageId        string        `json:"imageId"`
	Privileged     bool          `json:"privileged"`
	Restart        string        `json:"restart"`
	Cpus           int           `json:"cpus"`
	Memory         int           `json:"memory"`
	ShmSize        int           `json:"shmsize"`
	WorkDir        string        `json:"workDir"`
	User           string        `json:"user"`
	Command        string        `json:"command"`
	Entrypoint     string        `json:"entrypoint"`
	UseHostNetwork bool          `json:"useHostNetwork"`
	BindIpV6       bool          `json:"bindIpV6"`
}
