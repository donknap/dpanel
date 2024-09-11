package accessor

type SettingValueOption struct {
	Username       string                         `json:"username,omitempty"`
	Password       string                         `json:"password,omitempty"`
	ServerIp       string                         `json:"serverIp,omitempty"`
	RequestTimeout int                            `json:"requestTimeout,omitempty"`
	Docker         map[string]*DockerClientResult `json:"docker,omitempty"`
}

type DockerClientResult struct {
	Name      string `json:"name,omitempty"`
	Title     string `json:"title,omitempty"`
	Address   string `json:"address,omitempty"`
	Default   bool   `json:"default,omitempty"`
	TlsCa     string `json:"tlsCa,omitempty"`
	TlsCert   string `json:"tlsCert,omitempty"`
	TlsKey    string `json:"tlsKey,omitempty"`
	EnableTLS bool   `json:"enableTLS,omitempty"`
}
