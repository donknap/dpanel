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
		options: types.ServiceUpdateOptions{},
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
	serviceSpec swarm.ServiceSpec
	options     types.ServiceUpdateOptions
	Update      *swarm.Service
}

func (self *Builder) Execute() (string, []string, error) {
	if self.Update != nil {
		response, err := docker.Sdk.Client.ServiceUpdate(docker.Sdk.Ctx, self.Update.ID, self.Update.Version, self.serviceSpec, self.options)
		if err != nil {
			return self.Update.ID, nil, err
		}
		return self.Update.ID, response.Warnings, nil
	} else {
		response, err := docker.Sdk.Client.ServiceCreate(
			docker.Sdk.Ctx,
			self.serviceSpec,
			types.ServiceCreateOptions{
				EncodedRegistryAuth: self.options.EncodedRegistryAuth,
				QueryRegistry:       self.options.QueryRegistry,
			},
		)
		if err != nil {
			return "", nil, err
		}
		return response.ID, response.Warnings, nil
	}
}
