package accessor

import "time"

type SiteUpgradeLogsOption []SiteUpgradeLog

type SiteUpgradeLog struct {
	RunTime       time.Time                 `json:"runTime"`
	UseTime       float64                   `json:"useTime"`
	ExecutionType string                    `json:"executionType,omitempty"`
	Status        string                    `json:"status"`
	Error         string                    `json:"error,omitempty"`
	Containers    []SiteUpgradeLogContainer `json:"containers"`
}

type SiteUpgradeLogContainer struct {
	ContainerID    string `json:"containerId"`
	ContainerName  string `json:"containerName"`
	ImageName      string `json:"imageName"`
	Status         string `json:"status"`
	Error          string `json:"error,omitempty"`
	NewContainerID string `json:"newContainerId,omitempty"`
}
