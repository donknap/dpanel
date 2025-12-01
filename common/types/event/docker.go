package event

const (
	DockerStartEvent   = "docker_start"
	DockerDieEvent     = "docker_die"
	DockerMessageEvent = "docker_message"
)

type DockerPayload struct {
	Name string
}
