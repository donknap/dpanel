package accessor

type ComposeSettingOption struct {
	Status   string                   `json:"status,omitempty"`
	Type     string                   `json:"type"`
	Uri      []string                 `json:"uri,omitempty"`
	Override map[string]SiteEnvOption `json:"override,omitempty"`
}
