package controller

import (
	"github.com/donknap/dpanel/common/dao"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type Event struct {
	controller.Abstract
}

func (self Event) GetList(http *gin.Context) {
	type ParamsValidate struct {
		Page     int    `form:"page,default=1" binding:"omitempty,gt=0"`
		PageSize int    `form:"pageSize" binding:"omitempty"`
		Type     string `form:"type" binding:"omitempty,oneof=builder config container daemon image network node plugin secret service volume"`
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
	query := dao.Event.Order(dao.Event.ID.Desc())
	if params.Type != "" {
		query = query.Where(dao.Event.Type.Eq(params.Type))
	}
	list, total, _ := query.FindByPage((params.Page-1)*params.PageSize, params.PageSize)
	self.JsonResponseWithoutError(http, gin.H{
		"total": total,
		"page":  params.Page,
		"list":  list,
	})
	return
}

func (self Event) Prune(http *gin.Context) {
	oldRow, _ := dao.Event.Last()
	dao.Event.Where(dao.Event.ID.Lte(oldRow.ID)).Delete()
	self.JsonSuccessResponse(http)
	return
}
