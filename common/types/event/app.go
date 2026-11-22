package event

import "github.com/donknap/dpanel/common/service/docker/types"

const (
	AppBootstrap = "app_bootstrap"
	AppShutdown  = "app_shutdown"
)

type AppBootstrapPayload struct {
	DockerEnvList map[string]*types.DockerEnv `json:"dockerEnvList"`
}
