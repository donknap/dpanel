package migrate

import (
	"github.com/donknap/dpanel/common/dao"
)

type Upgrade20240909 struct{}

func (self Upgrade20240909) Version() string {
	return "1.1.1"
}

func (self Upgrade20240909) Upgrade() error {
	composeList, _ := dao.Compose.Find()
	for _, compose := range composeList {
		if compose.Setting == nil || compose.Setting.Status == "" {
			continue
		}
		compose.Setting.Status = ""
		_, err := dao.Compose.Updates(compose)
		if err != nil {
			return err
		}
	}
	return nil
}
