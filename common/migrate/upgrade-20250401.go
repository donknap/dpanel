package migrate

import (
	"github.com/donknap/dpanel/common/function"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

type Upgrade20250401 struct{}

func (self Upgrade20250401) Version() string {
	return "1.6.3"
}

func (self Upgrade20250401) Upgrade() error {
	// 如果获取不到数据，尝试将 container_info 中的数据变更成 json 串
	db, err := facade.GetDbFactory().Channel("default")
	if err != nil {
		return err
	}

	var ids []int32
	db.Table("ims_site").Select("id").Where("LENGTH(container_info) = ?", 64).Debug().Pluck("id", &ids)
	if !function.IsEmptyArray(ids) {
		db.Exec(`UPDATE ims_site SET container_info = CONCAT('{"Id": "', container_info, '"}') WHERE id IN (?)`, ids)
	}

	return nil
}
