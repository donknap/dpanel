package controller

import (
	logic2 "github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type Tag struct {
	controller.Abstract
}

func (self Tag) Create(http *gin.Context) {
	type ParamsValidate struct {
		Tag             string `json:"tag" binding:"required"`
		TagColor        string `json:"tagColor"`
		EnableShowGroup bool   `json:"enableShowGroup"`
		accessor.TagItem
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	// 非容器时生成一个Id串
	if params.Name == "" {
		params.Name = uuid.New().String()
	}
	tagList := make([]accessor.Tag, 0)
	logic2.Setting{}.GetByKey(logic2.SettingGroupSetting, logic2.SettingGroupSettingTag, &tagList)

	if ok, i := function.IndexArrayWalk(tagList, func(i accessor.Tag) bool {
		if i.Tag == params.Tag {
			return true
		}
		return false
	}); ok {
		if ok, j := function.IndexArrayWalk(tagList[i].Item, func(item accessor.TagItem) bool {
			return item.Name != "" && item.Name == params.Name
		}); ok {
			tagList[i].Item[j] = params.TagItem
		} else {
			tagList[i].Item = append(tagList[i].Item, params.TagItem)
		}
		tagList[i].TagColor = params.TagColor
		tagList[i].EnableShowGroup = params.EnableShowGroup
	} else {
		tagList = append(tagList, accessor.Tag{
			EnableShowGroup: params.EnableShowGroup,
			Tag:             params.Tag,
			TagColor:        params.TagColor,
			Item: []accessor.TagItem{
				params.TagItem,
			},
		})
	}
	_ = logic2.Setting{}.Save(&entity.Setting{
		GroupName: logic2.SettingGroupSetting,
		Name:      logic2.SettingGroupSettingTag,
		Value: &accessor.SettingValueOption{
			Tag: tagList,
		},
	})
	self.JsonSuccessResponse(http)
	return
}

func (self Tag) GetList(http *gin.Context) {
	type ParamsValidate struct {
		Name string `json:"name"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	tagList := make([]accessor.Tag, 0)
	logic2.Setting{}.GetByKey(logic2.SettingGroupSetting, logic2.SettingGroupSettingTag, &tagList)

	if params.Name != "" {
		tagList = function.PluckArrayWalk(tagList, func(item accessor.Tag) (accessor.Tag, bool) {
			if !function.IsEmptyArray(item.Item) && function.InArrayWalk(item.Item, func(i accessor.TagItem) bool {
				return i.Name == params.Name
			}) {
				return item, true
			}
			return item, false
		})
	}
	self.JsonResponseWithoutError(http, gin.H{
		"list": tagList,
	})
	return
}

func (self Tag) Delete(http *gin.Context) {
	type ParamsValidate struct {
		Name string `json:"name" binding:"required"`
		Tag  string `json:"tag"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	tagList := make([]accessor.Tag, 0)
	logic2.Setting{}.GetByKey(logic2.SettingGroupSetting, logic2.SettingGroupSettingTag, &tagList)

	if params.Tag != "" {
		tagList = function.PluckArrayWalk(tagList, func(item accessor.Tag) (accessor.Tag, bool) {
			if item.Tag == params.Tag {
				if ok, i := function.IndexArrayWalk(item.Item, func(i accessor.TagItem) bool {
					if i.Name == params.Name {
						return true
					}
					return false
				}); ok {
					item.Item = append(item.Item[:i], item.Item[i+1:]...)
				}
				if function.IsEmptyArray(item.Item) {
					return item, false
				}
				return item, true
			} else {
				return item, true
			}
		})
	} else {
		tagList = function.PluckArrayWalk(tagList, func(item accessor.Tag) (accessor.Tag, bool) {
			if ok, i := function.IndexArrayWalk(item.Item, func(i accessor.TagItem) bool {
				if i.Name == params.Name {
					return true
				}
				return false
			}); ok {
				item.Item = append(item.Item[:i], item.Item[i+1:]...)
			}
			if function.IsEmptyArray(item.Item) {
				return item, false
			}
			return item, true
		})
	}

	_ = logic2.Setting{}.Save(&entity.Setting{
		GroupName: logic2.SettingGroupSetting,
		Name:      logic2.SettingGroupSettingTag,
		Value: &accessor.SettingValueOption{
			Tag: tagList,
		},
	})
	self.JsonSuccessResponse(http)
}
