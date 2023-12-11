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

func (self Event) Unread(http *gin.Context, lastEvent *entity.Event) {
	type ParamsValidate struct {
		Show bool `form:"show" binding:"omitempty"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	var total int64
	var list []*entity.Event
	list, total, _ = dao.Event.Order(dao.Event.ID.Desc()).Where(dao.Event.ID.Gt(lastEvent.ID)).FindByPage(0, 10)

	if function.IsEmptyArray[*entity.Event](list) {
		list = make([]*entity.Event, 0)
	}
	self.JsonResponseWithoutError(http, gin.H{
		"list":        list,
		"unreadTotal": total,
	})
	return
}
