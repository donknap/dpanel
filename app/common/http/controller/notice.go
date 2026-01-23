package controller

import (
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"gorm.io/gorm"
)

type Notice struct {
	controller.Abstract
}

func (self Notice) Unread(http *gin.Context) {
	type ParamsValidate struct {
		Action string `json:"action" binding:"required,oneof=new clear init"`
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
	if function.IsEmptyArray(list) {
		list = make([]*entity.Notice, 0)
	}
	if params.Action == "clear" {
		db, err := facade.GetDbFactory().Channel("default")
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&entity.Notice{})
	}
	self.JsonResponseWithoutError(http, gin.H{
		"list":        list,
		"unreadTotal": total,
	})
	return
}

func (self Notice) GetList(http *gin.Context) {
	type ParamsValidate struct {
		Page     int    `json:"page" binding:"omitempty,gt=0"`
		PageSize int    `json:"pageSize" binding:"omitempty"`
		Type     string `json:"type" binding:"omitempty"`
	}

	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 10
	}
	query := dao.Notice.Order(dao.Notice.ID.Desc())
	if params.Type != "" {
		query = query.Where(dao.Notice.Title.Eq(params.Type))
	}
	list, total, _ := query.FindByPage((params.Page-1)*params.PageSize, params.PageSize)
	self.JsonResponseWithoutError(http, gin.H{
		"total": total,
		"page":  params.Page,
		"list":  list,
	})
	return
}

func (self Notice) Delete(http *gin.Context) {
	type ParamsValidate struct {
		Id []int32 `json:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	dao.Notice.Where(dao.Notice.ID.In(params.Id...)).Delete()
	self.JsonSuccessResponse(http)
	return
}
