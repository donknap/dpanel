package plugin

type pluginSetting struct {
	Name      string            `json:"name"`
	Image     map[string]string `json:"image"`
	ImageName string            `json:"imageName"`
	Env       struct {
		PidHost    bool   `json:"pidHost"`
		Privileged bool   `json:"privileged"`
		Restart    string `json:"restart"`
		AutoRemove bool   `json:"autoRemove"`
	} `json:"env"`
	Container struct {
		Init string `json:"init"`
	} `json:"container"`
}

type AttachOption struct {
	WorkingDir string
}
