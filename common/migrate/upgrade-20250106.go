package migrate

import (
	"log/slog"

	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/service/docker"
	"gorm.io/datatypes"
	"gorm.io/gen"
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
		err := dao.Compose.Save(compose)
		if err != nil {
			return err
		}
	}
	query := dao.Compose.Where(gen.Cond(
		datatypes.JSONQuery("setting").Equals("outPath", "type"),
	)...)
	if list, err := query.Find(); err == nil && list != nil && len(list) > 0 {
		ids := make([]int32, 0)
		for _, compose := range list {
			ids = append(ids, compose.ID)
		}
		slog.Debug("clear outPath db record", "ids", ids)
		_, err := dao.Compose.Where(dao.Compose.ID.In(ids...)).Delete()
		if err != nil {
			return err
		}
	}
	return nil
}
