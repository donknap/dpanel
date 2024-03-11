package controller

import (
	"errors"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
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
	oldUser, err := logic.Setting{}.GetValue(logic.SettingUser, logic.SettingUserFounder)
	if err != nil {
		self.JsonResponseWithError(http, errors.New("创始人配置不存在，请重新安装"), 500)
		return
	}

	// 修改密码
	if params.Password != "" && params.NewPassword != "" {
		if (*oldUser.Value)["password"] != function.GetMd5(params.Password+(*oldUser.Value)["username"]) {
			self.JsonResponseWithError(http, errors.New("旧密码不正确"), 500)
			return
		}
		(*oldUser.Value)["password"] = function.GetMd5(params.NewPassword + (*oldUser.Value)["username"])
	}

	if params.Username != "" {
		(*oldUser.Value)["username"] = params.Username
		if params.NewPassword == "" {
			(*oldUser.Value)["password"] = function.GetMd5(params.Password + (*oldUser.Value)["username"])
		} else {
			(*oldUser.Value)["password"] = function.GetMd5(params.NewPassword + (*oldUser.Value)["username"])
		}
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
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"setting": row,
	})
	return

}
