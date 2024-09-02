package controller

import (
	"errors"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type Setting struct {
	controller.Abstract
}

func (self Setting) Founder(http *gin.Context) {
	type ParamsValidate struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		NewPassword string `json:"newPassword"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	oldUser, err := logic.Setting{}.GetValue(logic.SettingGroupUser, logic.SettingGroupUserFounder)
	if err != nil {
		self.JsonResponseWithError(http, errors.New("创始人配置不存在，请重新安装"), 500)
		return
	}
	if oldUser.Value.Password != function.GetMd5(params.Password+oldUser.Value.Username) {
		self.JsonResponseWithError(http, errors.New("旧密码不正确"), 500)
		return
	}

	// 修改密码
	if params.NewPassword != "" {
		oldUser.Value.Password = function.GetMd5(params.NewPassword + oldUser.Value.Username)
		params.Password = params.NewPassword
	}

	// 修改用户名
	if params.Username != "" {
		oldUser.Value.Username = params.Username
		oldUser.Value.Password = function.GetMd5(params.Password + params.Username)
	}

	err = logic.Setting{}.Save(oldUser)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Setting) Save(http *gin.Context) {
	type ParamsValidate struct {
		GroupName string                      `json:"groupName" binding:"required"`
		Name      string                      `json:"name" binding:"required"`
		Value     accessor.SettingValueOption `json:"value" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	settingRow := &entity.Setting{
		GroupName: params.GroupName,
		Name:      params.Name,
		Value:     &params.Value,
	}
	err := logic.Setting{}.Save(settingRow)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Setting) GetSetting(http *gin.Context) {
	type ParamsValidate struct {
		GroupName string `json:"groupName" binding:"required"`
		Name      string `json:"name" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	row, err := logic.Setting{}.GetValue(params.GroupName, params.Name)
	if err != nil {
		self.JsonResponseWithoutError(http, gin.H{
			"setting": &entity.Setting{
				GroupName: params.GroupName,
				Name:      params.Name,
				Value:     &accessor.SettingValueOption{},
			},
		})
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"setting": row,
	})
	return

}
