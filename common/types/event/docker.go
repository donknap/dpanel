package event

import (
	"github.com/docker/docker/api/types/events"
	"github.com/donknap/dpanel/common/service/docker/types"
	types2 "github.com/donknap/dpanel/common/types"
)

const (
	DockerDaemonEvent = "docker_daemon"
)

type DockerDaemonPayload struct {
	DockerEnvName string             `json:"dockerEnvName"`
	Status        types.DockerStatus `json:"status"`
}

const (
	DockerMessageEvent = "docker_message"
)

type DockerMessagePayload struct {
	ID            string          `json:"id"`
	DockerEnvName string          `json:"dockerEnvName"`
	Level         types2.LogLevel `json:"level"`
	Message       events.Message  `json:"message"`
}
