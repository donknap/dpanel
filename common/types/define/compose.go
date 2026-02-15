package define

const (
	ComposeLabelProject     = "com.docker.compose.project"
	ComposeLabelService     = "com.docker.compose.service"
	ComposeLabelConfigFiles = "com.docker.compose.project.config_files"
	ComposeLabelConfigHash  = "com.docker.compose.config-hash"

	DPanelLabelComposeProject      = "com.dpanel.compose.project"
	DPanelLabelContainerAutoRemove = "com.dpanel.container.auto_remove"
	DPanelLabelContainerTitle      = "com.dpanel.container.title"
	DPanelLabelContainerHidden     = "com.dpanel.container.hidden"
	DPanelLabelContainerDPanelSelf = "com.dpanel.container.dpanel_self" // 表示当前容器为 DPanel 自身限制管理

	ComposeProjectPrefix = "dpanel-c-"                 // Deprecated
	ComposeProjectName   = ComposeProjectPrefix + "%s" // Deprecated

	ComposeProjectDeployFileName                = "dpanel-deploy.yaml" // Deprecated
	ComposeProjectDeployOverrideFileName        = "dpanel-override.yaml"
	ComposeProjectDeployOverrideOutPathFileName = "dpanel-%s-override.yaml"
	ComposeProjectDeployComposeFileName         = "docker-compose.yaml"
	ComposeDefaultEnvFileName                   = ".env"
)
