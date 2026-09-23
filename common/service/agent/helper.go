package agent

import (
	"github.com/donknap/dpanel/common/service/agent/runner"
	"github.com/donknap/dpanel/common/service/docker"
)

func NewDockerAgent(dockerSdk *docker.Client, containerName string) (*Agent, error) {
	dockerRunner, err := runner.NewDocker(dockerSdk, containerName)
	if err != nil {
		return nil, err
	}
	return NewAgent(dockerRunner)
}
