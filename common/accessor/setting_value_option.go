package accessor

type SettingValueOption struct {
	Username       string                         `json:"username,omitempty"`
	Password       string                         `json:"password,omitempty"`
	ServerIp       string                         `json:"serverIp,omitempty"`
	RequestTimeout int                            `json:"requestTimeout,omitempty"`
	Docker         map[string]*DockerClientResult `json:"docker,omitempty"`
}

type DockerClientResult struct {
	Name    string `json:"name"`
	Title   string `json:"title"`
	Address string `json:"address"`
	Default bool   `json:"default"`
}
