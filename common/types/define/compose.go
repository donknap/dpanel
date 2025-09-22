package define

const (
	ComposeLabelProject     = "com.docker.compose.project"
	ComposeLabelService     = "com.docker.compose.service"
	ComposeLabelConfigFiles = "com.docker.compose.project.config_files"
	ComposeLabelConfigHash  = "com.docker.compose.config-hash"

	DPanelLabelComposeProject      = "com.dpanel.compose.project"
	DPanelLabelContainerAutoRemove = "com.dpanel.container.auto_remove"
	DPanelLabelContainerTitle      = "com.dpanel.container.title"

	// Deprecated
	ComposeProjectPrefix = "dpanel-c-"
	// Deprecated
	ComposeProjectName = ComposeProjectPrefix + "%s"

	ComposeProjectDeployFileName                = "dpanel-deploy.yaml"
	ComposeProjectDeployOverrideFileName        = "dpanel-override.yaml"
	ComposeProjectDeployOverrideOutPathFileName = "dpanel-%s-override.yaml"
	ComposeDefaultEnvFileName                   = ".env"
)
