package migrate

import (
	"github.com/donknap/dpanel/common/function"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

type Upgrade20250521 struct{}

func (self Upgrade20250521) Version() string {
	return "1.7.0"
}

func (self Upgrade20250521) Upgrade() error {
	// 如果获取不到数据，尝试将 container_info 中的数据变更成 json 串
	db, err := facade.GetDbFactory().Channel("default")
	if err != nil {
		return err
	}

	replaceName := []map[string]string{
		{
			"old": "Tag",
			"new": "tag",
		},
		{
			"old": "checkContainerAllUpgrade",
			"new": "containerCheckAllUpgrade",
		},
		{
			"old": "checkContainerIgnore",
			"new": "containerCheckIgnoreUpgrade",
		},
	}

	for _, item := range replaceName {
		var ids []int32
		db.Table("ims_setting").Where("group_name = ? AND name = ?", "setting", item["old"]).Pluck("id", &ids)
		if !function.IsEmptyArray(ids) {
			db.Exec(`UPDATE ims_setting SET name = ? WHERE id IN (?)`, item["new"], ids)
		}
	}

	replaceFieldName := []map[string]string{
		{
			"old": "themeUser",
			"new": "themeUserConfig",
		},
		{
			"old": "theme",
			"new": "themeConfig",
		},
		{
			"old": "ignoreCheckUpgrade",
			"new": "containerCheckIgnoreUpgrade",
		},
	}

	for _, item := range replaceFieldName {
		var ids []int32
		db.Table("ims_setting").Where("group_name = ? AND name = ? AND value NOT LIKE ?;", "setting", item["new"], "%\""+item["new"]+"\"%").Pluck("id", &ids)
		if !function.IsEmptyArray(ids) {
			db.Exec(`UPDATE ims_setting SET value = REPLACE(value, ?, ?) WHERE id IN (?)`, item["old"], item["new"], ids)
		}
	}

	return nil
}
