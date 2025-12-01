package container

import (
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/types/define"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

func New(opts ...Option) (*Builder, error) {
	var err error
	c := &Builder{
		containerConfig: &container.Config{
			ExposedPorts: make(nat.PortSet),
			Labels: map[string]string{
				"maintainer":             define.PanelAuthor,
				"com.dpanel.description": define.PanelDesc,
				"com.dpanel.website":     define.PanelWebSite,
				"com.dpanel.version":     facade.GetConfig().GetString("app.version"),
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

func (self *Builder) GetConfig() (*container.Config, *container.HostConfig, *network.NetworkingConfig) {
	return self.containerConfig, self.hostConfig, self.networkingConfig
}
