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

type SiteEnvOption struct {
	Environment []EnvItem     `json:"environment"`
	Links       []LinkItem    `json:"links"`
	Ports       []MappingItem `json:"ports"`
	Volumes     []MappingItem `json:"volumes"`
}
