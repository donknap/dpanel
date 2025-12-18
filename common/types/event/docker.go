package event

import (
	"github.com/donknap/dpanel/common/service/docker/types"
)

const (
	DockerDaemonEvent  = "docker_daemon"
	DockerMessageEvent = "docker_message"
)

type DockerDaemonPayload struct {
	DockerEnv *types.DockerEnv
	Status    types.DockerStatus
}

type DockerMessagePayload struct {
	Type    string
	Action  string
	Time    int64
	Message []string
}
