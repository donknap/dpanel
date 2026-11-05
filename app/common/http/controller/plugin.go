package controller

import (
	"sort"
	"strings"

	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"gorm.io/datatypes"
	"gorm.io/gen"
)

type Plugin struct {
	controller.Abstract
}

func (self Plugin) GetList(http *gin.Context) {
	var params struct {
		StoreId int32  `json:"storeId" binding:"gte=0"`
		Keyword string `json:"keyword"`
	}
	if !self.Validate(http, &params) {
		return
	}
	query := dao.Store.Where(dao.Store.Name.Eq(define.StoreSystemPluginName)).Where(gen.Cond(
		datatypes.JSONQuery("setting").Equals(define.StoreTypeDPanel, "type"),
	)...)
	if params.StoreId > 0 {
		query = query.Where(dao.Store.ID.Eq(params.StoreId))
	}
	storeRow, err := query.First()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	appList := make([]accessor.StoreAppDetail, 0)
	keyword := strings.ToLower(strings.TrimSpace(params.Keyword))
	for _, app := range storeRow.Setting.Apps {
		if keyword != "" && !strings.Contains(strings.ToLower(app.Name), keyword) && !strings.Contains(strings.ToLower(app.Title), keyword) {
			match := false
			for _, title := range app.Titles {
				if strings.Contains(strings.ToLower(title), keyword) {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		appList = append(appList, app)
	}
	sort.Slice(appList, func(i, j int) bool {
		return appList[i].Name < appList[j].Name
	})
	storeRow.Setting.Apps = appList
	self.JsonResponseWithoutError(http, gin.H{"info": storeRow})
}
