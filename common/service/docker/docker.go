package docker

import (
	"context"
	"errors"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"strings"
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

// ContainerByField 获取单条容器 field 支持 id,name
func (self Builder) ContainerByField(field string, name ...string) (container map[string]*types.Container, err error) {
	ctx := context.Background()
	if len(name) == 0 {
		return nil, errors.New("Please specify a container name")
	}
	filters := filters.NewArgs()

	for _, value := range name {
		filters.Add(field, value)
	}

	filters.Add("status", "created")
	filters.Add("status", "restarting")
	filters.Add("status", "running")
	filters.Add("status", "removing")
	filters.Add("status", "paused")
	filters.Add("status", "exited")
	filters.Add("status", "dead")

	containerList, err := self.Client.ContainerList(ctx, types.ContainerListOptions{
		Filters: filters,
	})
	if err != nil {
		return nil, err
	}
	if len(containerList) == 0 {
		return nil, errors.New("container not found")
	}
	container = make(map[string]*types.Container)
	for _, value := range containerList {
		temp := value
		key := strings.Trim(temp.Names[0], "/")
		container[key] = &temp
	}
	return container, nil
}
