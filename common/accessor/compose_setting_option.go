package accessor

type ComposeSettingOption struct {
	Yaml        string    `json:"yaml"`
	RawYaml     string    `json:"rawYaml"`
	Environment []EnvItem `json:"environment"`
}
