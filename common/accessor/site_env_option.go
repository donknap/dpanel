package accessor

type MappingItem struct {
	Host       string `json:"host"`
	Dest       string `json:"dest"`
	Permission string `json:"permission"`
}

type LinkItem struct {
	Name  string `json:"name"`
	Alise string `json:"alise"`
}

type EnvItem struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type PortItem struct {
	Type string `json:"type"`
	Host string `json:"host"`
	Dest string `json:"dest"`
}

type ImageItem struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (self ImageItem) GetImage() string {
	if self.Version != "" {
		return self.Name + ":" + self.Version
	} else {
		return self.Name
	}
}

type SiteEnvOption struct {
	Environment    []EnvItem     `json:"environment"`
	Links          []LinkItem    `json:"links"`
	Ports          []PortItem    `json:"ports"`
	Volumes        []MappingItem `json:"volumes"`
	VolumesDefault []MappingItem `json:"volumesDefault"`
	ImageName      string        `json:"imageName"`
	Privileged     bool          `json:"privileged"`
	Restart        string        `json:"restart"`
}
