package plugin

type pluginSetting struct {
	Name      string `json:"name"`
	Image     string `json:"image"`
	ImageName string `json:"imageName"`
	Env       struct {
		PidHost    bool   `json:"pidHost"`
		Privileged bool   `json:"privileged"`
		Restart    string `json:"restart"`
		AutoRemove bool   `json:"autoRemove"`
	} `json:"env"`
}

type AttachOption struct {
	WorkingDir string
}
