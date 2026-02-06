package controller

import (
	"fmt"
	"strings"

	"github.com/docker/docker/api/types/container"
	logic2 "github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/types/define"
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

	if i, ok := function.IndexArrayWalk(tagList, func(i accessor.Tag) bool {
		if i.Tag == params.Tag {
			return true
		}
		return false
	}); ok {
		if j, ok := function.IndexArrayWalk(tagList[i].Item, func(item accessor.TagItem) bool {
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
		ShowCompose bool `json:"showCompose"`
		ShowSwarm   bool `json:"showSwarm"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	tagList := make([]accessor.Tag, 0)
	logic2.Setting{}.GetByKey(logic2.SettingGroupSetting, logic2.SettingGroupSettingTag, &tagList)

	if params.ShowSwarm || params.ShowCompose {
		if containerList, err := docker.Sdk.Client.ContainerList(docker.Sdk.Ctx, container.ListOptions{All: true}); err == nil {
			labels := make([]string, 0)
			tagList = append(tagList, function.PluckArrayWalk(containerList, func(item container.Summary) (accessor.Tag, bool) {
				if v, ok := item.Labels[define.ComposeLabelProject]; ok && params.ShowCompose && !function.InArray(labels, v) {
					labels = append(labels, v)
					return accessor.Tag{
						Tag:   fmt.Sprintf("%s", strings.TrimPrefix(v, define.ComposeProjectPrefix)),
						Group: "compose",
					}, true
				}
				if v, ok := item.Labels[define.SwarmLabelService]; ok && params.ShowSwarm && !function.InArray(labels, v) {
					labels = append(labels, v)
					return accessor.Tag{
						Tag:   fmt.Sprintf("%s", v),
						Group: "swarm",
					}, true
				}
				return accessor.Tag{}, false
			})...)
		}
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
				if i, ok := function.IndexArrayWalk(item.Item, func(i accessor.TagItem) bool {
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
			if i, ok := function.IndexArrayWalk(item.Item, func(i accessor.TagItem) bool {
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
