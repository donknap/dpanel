package accessor

type MappingItem struct {
	Host string `json:"host"`
	Dest string `json:"dest"`
}

type LinkItem struct {
	Name  string `json:"name"`
	Alise string `json:"alise"`
}

type EnvItem struct {
	Name  string `json:"name"`
	Value string `json:"value"`
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
	Environment []EnvItem     `json:"environment"`
	Links       []LinkItem    `json:"links"`
	Ports       []MappingItem `json:"ports"`
	Volumes     []MappingItem `json:"volumes"`
	Image       ImageItem     `json:"image"`
}
