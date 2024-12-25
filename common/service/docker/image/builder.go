package image

import (
	"github.com/docker/docker/api/types"
	"github.com/donknap/dpanel/common/service/docker"
	"io"
)

func New(opts ...Option) (*Builder, error) {
	var err error
	c := &Builder{
		imageBuildOption: types.ImageBuildOptions{
			Dockerfile: "Dockerfile", // 默认在根目录
			Remove:     true,
			NoCache:    true,
			Labels: map[string]string{
				"BuildAuthor":  docker.BuilderAuthor,
				"BuildDesc":    docker.BuildDesc,
				"BuildWebSite": docker.BuildWebSite,
				"buildVersion": docker.BuildVersion,
			},
			BuildArgs: map[string]*string{},
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
	imageBuildOption types.ImageBuildOptions
	buildContext     io.Reader
}

func (self *Builder) Execute() (response types.ImageBuildResponse, err error) {
	response, err = docker.Sdk.Client.ImageBuild(
		docker.Sdk.Ctx,
		self.buildContext,
		self.imageBuildOption,
	)
	if err != nil {
		return response, err
	}
	return response, nil
}
