package runner

import (
	"context"
	"errors"
	serviceDocker "github.com/donknap/dpanel/common/service/docker"
	containerexec "github.com/donknap/dpanel/common/service/exec/container"
)

const dockerExecutable = "/agent"

type Docker struct {
	dockerSdk     *serviceDocker.Client
	containerName string
}

func NewDocker(dockerSdk *serviceDocker.Client, containerName string) (*Docker, error) {
	if dockerSdk == nil || dockerSdk.Client == nil {
		return nil, errors.New("docker client is required for agent runner")
	}
	if containerName == "" {
		return nil, errors.New("container name is required for agent runner")
	}
	return &Docker{dockerSdk: dockerSdk, containerName: containerName}, nil
}

func (self *Docker) Run(ctx context.Context, args ...string) ([]byte, error) {
	command := append([]string{dockerExecutable}, args...)
	return containerexec.QuickRun(ctx, self.dockerSdk.Client, self.containerName, command...)
}

func (self *Docker) Stream(ctx context.Context, args ...string) (Session, error) {
	command := append([]string{dockerExecutable}, args...)
	cmd, err := containerexec.New(
		containerexec.WithDockerClient(self.dockerSdk.Client),
		containerexec.WithContainerName(self.containerName),
		containerexec.WithCommandName(command[0]),
		containerexec.WithArgs(command[1:]...),
		containerexec.WithCtx(ctx),
	)
	if err != nil {
		return nil, err
	}
	pipe, err := cmd.RunInPip()
	if err != nil {
		_ = cmd.Close()
		return nil, err
	}
	return pipe, nil
}
