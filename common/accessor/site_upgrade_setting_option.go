package accessor

type SiteUpgradeSettingOption struct {
	DockerEnvName          string                  `json:"dockerEnvName"`
	Disable                bool                    `json:"disable,omitempty"`
	Expression             []CronSettingExpression `json:"expression"`
	FilterField            string                  `json:"filterField"`
	FilterValues           []string                `json:"filterValues"`
	ExcludeContainerNames  []string                `json:"excludeContainerNames"`
	IncludeRestarting      bool                    `json:"includeRestarting"`
	IncludeStopped         bool                    `json:"includeStopped"`
	ExecutionType          string                  `json:"executionType"`
	EnableBak              bool                    `json:"enableBak"`
	EnableResetImageConfig bool                    `json:"enableResetImageConfig"`
}
