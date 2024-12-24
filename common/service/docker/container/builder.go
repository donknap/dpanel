package container

import (
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/donknap/dpanel/common/service/docker"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func New(opts ...Option) (*Builder, error) {
	var err error
	c := &Builder{
		containerConfig: &container.Config{
			ExposedPorts: make(nat.PortSet),
			Labels: map[string]string{
				"maintainer":             docker.BuilderAuthor,
				"com.dpanel.description": docker.BuildDesc,
				"com.dpanel.website":     docker.BuildWebSite,
			},
		},
		hostConfig: &container.HostConfig{
			PortBindings: make(nat.PortMap),
			NetworkMode:  "default",
		},
		platform: &v1.Platform{},
		networkingConfig: &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{},
		},
	}
	for _, opt := range opts {
		err = opt(c)
		if err != nil {
			return nil, err
		}
	}
	return c, nil
}

type Builder struct {
	containerConfig  *container.Config
	hostConfig       *container.HostConfig
	networkingConfig *network.NetworkingConfig
	platform         *v1.Platform
	containerName    string
}

func (self *Builder) Execute() (response container.CreateResponse, err error) {
	return docker.Sdk.Client.ContainerCreate(
		docker.Sdk.Ctx,
		self.containerConfig,
		self.hostConfig,
		self.networkingConfig,
		self.platform,
		self.containerName,
	)
}
