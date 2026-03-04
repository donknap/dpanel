package event

import (
	"github.com/docker/docker/api/types/events"
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
	DockerEnvName string         `json:"dockerEnvName"`
	Message       events.Message `json:"message"`
}
