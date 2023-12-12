package controller

import (
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
)

type Event struct {
	controller.Abstract
}

func (self Event) Unread(http *gin.Context) {
	type ParamsValidate struct {
		Show bool `form:"show" binding:"omitempty"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	var total int64
	var list []*entity.Event

	lastUnread, _ := dao.Event.Where(dao.Event.Read.Eq(0)).First()
	if lastUnread != nil {
		list, total, _ = dao.Event.Order(dao.Event.ID.Desc()).Where(dao.Event.ID.Gte(lastUnread.ID)).FindByPage(0, 10)
	}
	if function.IsEmptyArray[*entity.Event](list) {
		list = make([]*entity.Event, 0)
	}
	self.JsonResponseWithoutError(http, gin.H{
		"list":        list,
		"unreadTotal": total,
	})
	return
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

func (self Event) MarkRead(http *gin.Context) {
	type ParamsValidate struct {
		Id int32 `form:"id" binding:"omitempty"`
	}

	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	if params.Id > 0 {
		dao.Event.Where(dao.Event.ID.Eq(params.Id)).Update(dao.Event.Read, 1)
	} else {
		lastUnread, _ := dao.Event.Where(dao.Event.Read.Eq(0)).First()
		if lastUnread != nil {
			dao.Event.Where(dao.Event.ID.Gte(lastUnread.ID)).Update(dao.Event.Read, 1)
		}
	}

	self.JsonResponseWithoutError(http, gin.H{
		"status": 1,
	})
	return
}
