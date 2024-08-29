package accessor

type ComposeSettingOption struct {
	RawYaml     string    `json:"rawYaml"`
	Environment []EnvItem `json:"environment"`
	Status      string    `json:"status"`
}
