package event

import (
	"github.com/docker/docker/api/types/events"
	"github.com/donknap/dpanel/common/service/docker/types"
)

const (
	DockerDaemonEvent = "docker_daemon"
)

type DockerDaemonPayload struct {
	DockerEnvName string
	Status        types.DockerStatus
}

const (
	DockerMessageEvent = "docker_message"
)

type DockerMessagePayload struct {
	DockerEnvName string         `json:"dockerEnvName"`
	Message       events.Message `json:"message"`
}
