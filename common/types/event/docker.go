package event

import (
	"github.com/docker/docker/api/types/events"
	"github.com/donknap/dpanel/common/service/docker/types"
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

type DockerMessageLevel string

const (
	DockerMessageLevelNormal DockerMessageLevel = "normal"
	DockerMessageLevelHigh   DockerMessageLevel = "high"
)

type DockerMessagePayload struct {
	DockerEnvName string             `json:"dockerEnvName"`
	Level         DockerMessageLevel `json:"level"`
	Message       events.Message     `json:"message"`
}
