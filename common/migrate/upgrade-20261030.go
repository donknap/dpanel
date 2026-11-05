package migrate

import (
	"errors"

	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/types/define"
	"gorm.io/gorm"
)

type Upgrade20261030 struct{}

func (Upgrade20261030) Version() string {
	return "1.11.3"
}

func (Upgrade20261030) Upgrade() error {
	storeRow, err := dao.Store.Where(dao.Store.Name.Eq(define.StoreSystemPluginName)).First()
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if storeRow != nil && storeRow.Title != "" &&
		storeRow.Setting != nil && storeRow.Setting.Type == define.StoreTypeDPanel &&
		storeRow.Setting.Url != "" {
		return nil
	}
	if storeRow == nil {
		storeRow = &entity.Store{Name: define.StoreSystemPluginName}
	}
	if storeRow.Title == "" {
		storeRow.Title = "DPanel"
	}
	if storeRow.Setting == nil || storeRow.Setting.Type != define.StoreTypeDPanel {
		// 其它格式的来源和应用索引不能作为系统插件继续使用。
		storeRow.Setting = &accessor.StoreSettingOption{Type: define.StoreTypeDPanel}
	}
	if storeRow.Setting.Url == "" {
		storeRow.Setting.Url = "https://github.com/DPanel-dev/plugin.git"
	}
	return dao.Store.Save(storeRow)
}
