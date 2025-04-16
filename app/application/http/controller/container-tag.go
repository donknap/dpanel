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
	type ParamsValidate struct {
		Tag             string `json:"tag" binding:"required"`
		TagColor        string `json:"tagColor"`
		EnableShowGroup bool   `json:"enableShowGroup"`
		accessor.ContainerTagItem
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	if !self.Validate(http, &params) {
		return
	}
	containerTag := make([]accessor.ContainerTag, 0)
	logic2.Setting{}.GetByKey(logic2.SettingGroupSetting, logic2.SettingGroupSettingContainerTag, &containerTag)

	if ok, i := function.IndexArrayWalk(containerTag, func(i accessor.ContainerTag) bool {
		if i.Tag == params.Tag {
			return true
		}
		return false
	}); ok {
		if ok, j := function.IndexArrayWalk(containerTag[i].Container, func(item accessor.ContainerTagItem) bool {
			return item.ContainerName == params.ContainerName
		}); ok {
			containerTag[i].Container[j] = params.ContainerTagItem
		} else {
			containerTag[i].Container = append(containerTag[i].Container, params.ContainerTagItem)
		}
		containerTag[i].TagColor = params.TagColor
		containerTag[i].EnableShowGroup = params.EnableShowGroup
	} else {
		containerTag = append(containerTag, accessor.ContainerTag{
			EnableShowGroup: params.EnableShowGroup,
			Tag:             params.Tag,
			TagColor:        params.TagColor,
			Container: []accessor.ContainerTagItem{
				params.ContainerTagItem,
			},
		})
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
	type ParamsValidate struct {
		ContainerName string `json:"containerName"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	containerTag := make([]accessor.ContainerTag, 0)
	logic2.Setting{}.GetByKey(logic2.SettingGroupSetting, logic2.SettingGroupSettingContainerTag, &containerTag)

	if params.ContainerName != "" {
		containerTag = function.PluckArrayWalk(containerTag, func(item accessor.ContainerTag) (accessor.ContainerTag, bool) {
			if !function.IsEmptyArray(item.Container) && function.InArrayWalk(item.Container, func(i accessor.ContainerTagItem) bool {
				return i.ContainerName == params.ContainerName
			}) {
				return item, true
			}
			return item, false
		})
	}
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
	containerTag := make([]accessor.ContainerTag, 0)
	logic2.Setting{}.GetByKey(logic2.SettingGroupSetting, logic2.SettingGroupSettingContainerTag, &containerTag)

	containerTag = function.PluckArrayWalk(containerTag, func(item accessor.ContainerTag) (accessor.ContainerTag, bool) {
		if item.Tag == params.Tag {
			if ok, i := function.IndexArrayWalk(item.Container, func(i accessor.ContainerTagItem) bool {
				if i.ContainerName == params.ContainerName {
					return true
				}
				return false
			}); ok {
				item.Container = append(item.Container[:i], item.Container[i+1:]...)
			}
			if function.IsEmptyArray(item.Container) {
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
