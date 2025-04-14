package controller

import (
	logic2 "github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type Tag struct {
	controller.Abstract
}

func (self Tag) Create(http *gin.Context) {
	params := accessor.ContainerTagItem{}
	if !self.Validate(http, &params) {
		return
	}
	containerTag := make([]accessor.ContainerTagItem, 0)
	logic2.Setting{}.GetByKey(logic2.SettingGroupSetting, logic2.SettingGroupSettingContainerTag, &containerTag)

	if ok, i := function.IndexArrayWalk(containerTag, func(i accessor.ContainerTagItem) bool {
		if i.Tag == params.Tag {
			return true
		}
		return false
	}); ok {
		containerTag[i].ContainerName = append(containerTag[i].ContainerName, params.ContainerName...)
	} else {
		containerTag = append(containerTag, params)
	}
	_ = logic2.Setting{}.Save(&entity.Setting{
		GroupName: logic2.SettingGroupSetting,
		Name:      logic2.SettingGroupSettingContainerTag,
		Value: &accessor.SettingValueOption{
			ContainerTag: containerTag,
		},
	})
	self.JsonSuccessResponse(http)
	return
}

func (self Tag) GetList(http *gin.Context) {
	containerTag := make([]accessor.ContainerTagItem, 0)
	logic2.Setting{}.GetByKey(logic2.SettingGroupSetting, logic2.SettingGroupSettingContainerTag, &containerTag)
	self.JsonResponseWithoutError(http, gin.H{
		"list": containerTag,
	})
	return
}
