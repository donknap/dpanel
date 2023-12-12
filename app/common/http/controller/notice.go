package controller

import (
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go-support/src/facade"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
	"gorm.io/gorm"
)

type Notice struct {
	controller.Abstract
	lastNoticeId int32
}

func (self *Notice) Unread(http *gin.Context) {
	type ParamsValidate struct {
		Action string `form:"action" binding:"required,oneof=new clear init"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	var list []*entity.Notice
	var total int64
	if params.Action == "init" {
		list, total, _ = dao.Notice.Order(dao.Notice.ID.Desc()).FindByPage(0, 5)
	}
	if params.Action == "new" {
		list, total, _ = dao.Notice.Order(dao.Notice.ID.Asc()).Where(dao.Notice.ID.Gt(self.lastNoticeId)).FindByPage(0, 1)
	}

	if function.IsEmptyArray(list) {
		list = make([]*entity.Notice, 0)
	} else {
		self.lastNoticeId = list[0].ID
	}

	if params.Action == "clear" {
		db, err := facade.GetDbFactory().Channel("default")
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&entity.Notice{})
		self.lastNoticeId = 0
	}

	self.JsonResponseWithoutError(http, gin.H{
		"list":        list,
		"unreadTotal": total,
	})
	return
}
