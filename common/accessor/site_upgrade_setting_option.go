package accessor

type SiteUpgradeSettingOption struct {
	DockerEnvName          string                  `json:"dockerEnvName"`
	Disable                bool                    `json:"disable,omitempty"`
	Expression             []CronSettingExpression `json:"expression"`
	ContainerNames         []string                `json:"containerNames"`
	EnableBak              bool                    `json:"enableBak,omitempty"`
	EnableResetImageConfig bool                    `json:"enableResetImageConfig,omitempty"`
}
