package image

import (
	"context"
	"io"
	"log/slog"

	"github.com/docker/docker/api/types/build"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

func New(opts ...Option) (*Builder, error) {
	var err error
	c := &Builder{
		imageBuildOption: build.ImageBuildOptions{
			Dockerfile:  "Dockerfile", // 默认在根目录
			Remove:      true,
			ForceRemove: true,
			NoCache:     false,
			//Version:     build.BuilderBuildKit,
			Labels: map[string]string{
				"maintainer":             define.PanelAuthor,
				"com.dpanel.description": define.PanelDesc,
				"com.dpanel.website":     define.PanelWebSite,
				"com.dpanel.version":     facade.GetConfig().GetString("app.version"),
			},
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
	imageBuildOption build.ImageBuildOptions
	buildContext     io.Reader
	ctx              context.Context
}

func (self *Builder) Execute() (response build.ImageBuildResponse, err error) {
	slog.Debug("image build", "dockerfile", self.imageBuildOption.Dockerfile)
	response, err = docker.Sdk.Client.ImageBuild(
		self.ctx,
		self.buildContext,
		self.imageBuildOption,
	)
	if err != nil {
		return response, err
	}
	return response, nil
}
