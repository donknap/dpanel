package controller

import (
	logic2 "github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type ContainerTag struct {
	controller.Abstract
}

func (self ContainerTag) Create(http *gin.Context) {
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
		if !function.InArrayArray(containerTag[i].ContainerName, params.ContainerName...) {
			containerTag[i].ContainerName = append(containerTag[i].ContainerName, params.ContainerName...)
		}
		containerTag[i].TagColor = params.TagColor
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

func (self ContainerTag) GetList(http *gin.Context) {
	containerTag := make([]accessor.ContainerTagItem, 0)
	logic2.Setting{}.GetByKey(logic2.SettingGroupSetting, logic2.SettingGroupSettingContainerTag, &containerTag)
	self.JsonResponseWithoutError(http, gin.H{
		"list": containerTag,
	})
	return
}

func (self ContainerTag) Delete(http *gin.Context) {
	type ParamsValidate struct {
		ContainerName string `json:"containerName" binding:"required"`
		Tag           string `json:"tag" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	containerTag := make([]accessor.ContainerTagItem, 0)
	logic2.Setting{}.GetByKey(logic2.SettingGroupSetting, logic2.SettingGroupSettingContainerTag, &containerTag)

	containerTag = function.PluckArrayWalk(containerTag, func(item accessor.ContainerTagItem) (accessor.ContainerTagItem, bool) {
		if item.Tag == params.Tag {
			if ok, i := function.IndexArrayWalk(item.ContainerName, func(i string) bool {
				if i == params.ContainerName {
					return true
				}
				return false
			}); ok {
				item.ContainerName = append(item.ContainerName[:i], item.ContainerName[i+1:]...)
			}
			if function.IsEmptyArray(item.ContainerName) {
				return item, false
			}
			return item, true
		} else {
			return item, true
		}
	})

	_ = logic2.Setting{}.Save(&entity.Setting{
		GroupName: logic2.SettingGroupSetting,
		Name:      logic2.SettingGroupSettingContainerTag,
		Value: &accessor.SettingValueOption{
			ContainerTag: containerTag,
		},
	})
	self.JsonSuccessResponse(http)
}
