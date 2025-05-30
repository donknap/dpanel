package migrate

import (
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/service/docker"
)

type Upgrade20250113 struct{}

func (self Upgrade20250113) Version() string {
	return "1.5.3"
}

func (self Upgrade20250113) Upgrade() error {
	list, _ := dao.Cron.Find()
	for _, item := range list {
		if item.Setting == nil || item.Setting.DockerEnvName != "" {
			continue
		}
		item.Setting.DockerEnvName = docker.DefaultClientName
		err := dao.Cron.Save(item)
		if err != nil {
			return err
		}
	}
	return nil
}
