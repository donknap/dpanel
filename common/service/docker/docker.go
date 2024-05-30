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

var (
	Sdk, _                          = NewDockerClient()
	QueueDockerProgressMessage      = make(chan *Progress, 999)
	QueueDockerImageDownloadMessage = make(chan map[string]*ProgressDownloadImage, 999)
	BuilderAuthor                   = "DPanel"
	BuildDesc                       = "DPanel is an open source docker web management panel that provides image building and deployment"
	BuildWebSite                    = "https://github.com/donknap/dpanel, https://phpey.net"
	BuildVersion                    = "1.0.0"
	HostnameTemplate                = "%s.pod.dpanel.local"
)

type Builder struct {
	Client *client.Client
	Ctx    context.Context
}

func NewDockerClient() (*Builder, error) {
	obj, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
		//client.WithHost("unix:///var/run/docker.sock"),
	)
	if err != nil {
		return nil, err
	}

	return &Builder{
		Client: obj,
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
		option: container.LogsOptions{
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
func (self Builder) ContainerByField(field string, name ...string) (result map[string]*types.Container, err error) {
	if len(name) == 0 {
		return nil, errors.New("please specify a container name")
	}
	filtersArgs := filters.NewArgs()

	for _, value := range name {
		filtersArgs.Add(field, value)
	}

	filtersArgs.Add("status", "created")
	filtersArgs.Add("status", "restarting")
	filtersArgs.Add("status", "running")
	filtersArgs.Add("status", "removing")
	filtersArgs.Add("status", "paused")
	filtersArgs.Add("status", "exited")
	filtersArgs.Add("status", "dead")

	containerList, err := Sdk.Client.ContainerList(Sdk.Ctx, container.ListOptions{
		Filters: filtersArgs,
	})
	if err != nil {
		return nil, err
	}
	if len(containerList) == 0 {
		return nil, errors.New("container not found")
	}
	result = make(map[string]*types.Container)

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
		result[key] = &temp
	}
	return result, nil
}

func (self Builder) ContainerInfo(md5 string) (info types.ContainerJSON, err error) {
	info, _, err = Sdk.Client.ContainerInspectWithRaw(Sdk.Ctx, md5, true)
	if err != nil {
		return info, err
	}
	info.Name = strings.TrimPrefix(info.Name, "/")
	return info, nil
}

func (self Builder) GetRestartPolicyByString(restartType string) (mode container.RestartPolicyMode) {
	restartPolicyMap := map[string]container.RestartPolicyMode{
		"always":         container.RestartPolicyAlways,
		"no":             container.RestartPolicyDisabled,
		"unless-stopped": container.RestartPolicyUnlessStopped,
		"on-failure":     container.RestartPolicyOnFailure,
	}
	if mode, ok := restartPolicyMap[restartType]; ok {
		return mode
	} else {
		return container.RestartPolicyDisabled
	}
}
