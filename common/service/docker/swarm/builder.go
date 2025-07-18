package swarm

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/donknap/dpanel/common/service/docker"
)

func New(opts ...Option) (*Builder, error) {
	var err error
	c := &Builder{
		serviceSpec: swarm.ServiceSpec{
			Annotations: swarm.Annotations{
				Labels: map[string]string{
					"maintainer":             docker.BuilderAuthor,
					"com.dpanel.description": docker.BuildDesc,
					"com.dpanel.website":     docker.BuildWebSite,
				},
			},
			TaskTemplate: swarm.TaskSpec{
				ContainerSpec: &swarm.ContainerSpec{},
			},
			Mode:           swarm.ServiceMode{},
			UpdateConfig:   &swarm.UpdateConfig{},
			RollbackConfig: &swarm.UpdateConfig{},
			EndpointSpec:   &swarm.EndpointSpec{},
		},
		serviceCreateOptions: types.ServiceCreateOptions{},
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
	serviceSpec          swarm.ServiceSpec
	serviceCreateOptions types.ServiceCreateOptions
}

func (self *Builder) Execute() (response swarm.ServiceCreateResponse, err error) {
	return docker.Sdk.Client.ServiceCreate(
		docker.Sdk.Ctx,
		self.serviceSpec,
		self.serviceCreateOptions,
	)
}
