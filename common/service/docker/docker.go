package docker

import (
	"context"
	"encoding/json"
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

var (
	Sdk, _                          = NewDockerClient()
	QueueDockerProgressMessage      = make(chan *Progress, 999)
	QueueDockerImageDownloadMessage = make(chan map[string]*ProgressDownloadImage, 999)
	BuilderAuthor                   = "DPanel"
	BuildDesc                       = "DPanel is an open source docker web management panel that provides image building and deployment"
	BuildWebSite                    = "https://github.com/donknap/dpanel, https://phpey.net"
	BuildVersion                    = "1.0.0"
)

type Builder struct {
	Client *client.Client
	Ctx    context.Context
}

func NewDockerClient() (*Builder, error) {
	client, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
		//client.WithHost(facade.GetConfig().GetString("docker.sock")),
		client.WithHost("unix:///Users/renchao/.docker/run/docker.sock"),
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
		ctx:              context.Background(),
	}
	return builder
}

func (self Builder) GetContainerLogBuilder() *containerLogBuilder {
	builder := &containerLogBuilder{
		option: types.ContainerLogsOptions{
			Timestamps: false,
			ShowStderr: true,
			ShowStdout: true,
		},
	}
	builder.withSdk(self.Client)
	return builder
}

func (self Builder) GetImageBuildBuilder() *imageBuildBuilder {
	builder := &imageBuildBuilder{
		imageBuildOption: types.ImageBuildOptions{
			Dockerfile: "Dockerfile", // 默认在根目录
			Remove:     true,
			NoCache:    true,
			Labels: map[string]string{
				"BuildAuthor":  BuilderAuthor,
				"BuildDesc":    BuildDesc,
				"BuildWebSite": BuildWebSite,
				"buildVersion": BuildVersion,
			},
		},
	}
	return builder
}

// ContainerByField 获取单条容器 field 支持 id,name
func (self Builder) ContainerByField(field string, name ...string) (container map[string]*types.Container, err error) {
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

	containerList, err := Sdk.Client.ContainerList(Sdk.Ctx, types.ContainerListOptions{
		Filters: filters,
	})
	if err != nil {
		return nil, err
	}
	if len(containerList) == 0 {
		return nil, errors.New("container not found")
	}
	container = make(map[string]*types.Container)

	var key string
	for _, value := range containerList {
		temp := value
		if field == "name" {
			key = strings.Trim(temp.Names[0], "/")
		} else if field == "id" {
			key = value.ID
		} else {
			key = value.ID
		}
		container[key] = &temp
	}
	return container, nil
}

func (self Builder) ContainerInfoState(md5 string) (info *containerInfoState, err error) {
	_, jsonRaw, err := Sdk.Client.ContainerInspectWithRaw(Sdk.Ctx, md5, true)
	if err != nil {
		return info, err
	}
	err = json.Unmarshal(jsonRaw, &info)
	if err != nil {
		return info, nil
	}
	return info, nil
}
