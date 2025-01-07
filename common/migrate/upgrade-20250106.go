package migrate

import (
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/service/docker"
)

type Upgrade20250106 struct{}

func (self Upgrade20250106) Version() string {
	return "1.5.2"
}

func (self Upgrade20250106) Upgrade() error {
	composeList, _ := dao.Compose.Find()
	for _, compose := range composeList {
		if compose.Setting == nil || compose.Setting.DockerEnvName != "" {
			continue
		}
		compose.Setting.DockerEnvName = docker.DefaultClientName
		_, err := dao.Compose.Updates(compose)
		if err != nil {
			return err
		}
	}
	return nil
}
