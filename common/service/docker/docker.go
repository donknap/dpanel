package docker

import (
	"context"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type Builder struct {
	Client *client.Client
	Ctx    context.Context
}

func NewDockerClient() (*Builder, error) {
	client, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
		client.WithHost("unix:///Users/renchao/Library/Containers/com.docker.docker/Data/docker.raw.sock"),
	)
	if err != nil {
		return nil, err
	}

	return &Builder{
		Client: client,
		Ctx:    context.Background(),
	}, nil
}

func (self Builder) GetContainerCreateBuilder() *ContainerCreateBuilder {
	builder := &ContainerCreateBuilder{
		containerConfig: &container.Config{
			ExposedPorts: make(nat.PortSet),
		},
		hostConfig: &container.HostConfig{
			PortBindings: make(nat.PortMap),
			NetworkMode:  "default",
		},
		platform:         &v1.Platform{},
		networkingConfig: &network.NetworkingConfig{},
	}
	builder.withSdk(self.Client)
	return builder
}
