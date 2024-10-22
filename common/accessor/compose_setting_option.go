package accessor

type ComposeSettingOption struct {
	Environment []EnvItem                `json:"environment"`
	Status      string                   `json:"status"`
	Type        string                   `json:"type"`
	Uri         string                   `json:"uri,omitempty"`
	Override    map[string]SiteEnvOption `json:"override,omitempty"`
}
