package event

import (
	"github.com/donknap/dpanel/common/service/docker/types"
)

const (
	DockerStartEvent   = "docker_start"
	DockerStopEvent    = "docker_stop"
	DockerMessageEvent = "docker_message"
)

type DockerPayload struct {
	DockerEnv *types.DockerEnv
	Error     error
}

type DockerMessagePayload struct {
	Type    string
	Action  string
	Time    int64
	Message []string
}
