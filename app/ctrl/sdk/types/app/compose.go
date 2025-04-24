package app

import (
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/service/compose"
	"github.com/donknap/dpanel/common/service/docker"
)

type ComposeDeployOption struct {
	Id                string           `json:"id" binding:"required"`
	Environment       []docker.EnvItem `json:"environment"`
	DeployServiceName []string         `json:"deployServiceName"`
	CreatePath        bool             `json:"createPath"`
	RemoveOrphans     bool             `json:"removeOrphans"`
}

type ComposeDetailResult struct {
	Detail        *entity.Compose            `json:"detail"`
	Yaml          [2]string                  `json:"yaml"`
	ContainerList []*compose.ContainerResult `json:"containerList"`
	Project       struct {
		Name     string `json:"name"`
		Services map[string]struct {
			Image string `json:"image"`
		} `json:"services"`
	} `json:"project"`
}
