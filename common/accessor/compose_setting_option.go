package accessor

type ComposeSettingOption struct {
	RawYaml     string    `json:"rawYaml,omitempty"`
	Environment []EnvItem `json:"environment"`
	Status      string    `json:"status"`
	Type        string    `json:"type"`
	Uri         string    `json:"uri,omitempty"`
}
