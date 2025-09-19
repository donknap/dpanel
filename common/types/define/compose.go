package define

const (
	ComposeLabelProject       = "com.docker.compose.project"
	ComposeLabelService       = "com.docker.compose.service"
	ComposeLabelConfigFiles   = "com.docker.compose.project.config_files"
	ComposeLabelConfigHash    = "com.docker.compose.config-hash"
	ComposeLabelDPanelProject = "com.dpanel.compose.project"

	// Deprecated
	ComposeProjectPrefix = "dpanel-c-"
	ComposeProjectName   = ComposeProjectPrefix + "%s"

	ComposeProjectDeployFileName = "dpanel-deploy.yaml"
	ComposeDefaultEnvFileName    = ".env"
)
