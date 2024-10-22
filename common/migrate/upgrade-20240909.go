package migrate

import (
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
)

type Upgrade20240909 struct{}

func (self Upgrade20240909) Version() string {
	return "1.1.1"
}

func (self Upgrade20240909) Upgrade() error {
	composeList, _ := dao.Compose.Find()
	for _, compose := range composeList {
		if compose.Setting != nil {
			continue
		}
		_, err := dao.Compose.Where(dao.Compose.ID.Eq(compose.ID)).Updates(&entity.Compose{
			Setting: &accessor.ComposeSettingOption{
				Status:      "waiting",
				Environment: make([]accessor.EnvItem, 0),
			},
		})
		if err != nil {
			return err
		}
	}
	return nil
}
