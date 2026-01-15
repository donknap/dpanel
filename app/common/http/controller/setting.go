package controller

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/donknap/dpanel/common/types/event"
	"github.com/gin-gonic/gin"
	"github.com/patrickmn/go-cache"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
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
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}
	if oldUser.Value.Password != function.GetMd5(params.Password+oldUser.Value.Username) {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageUserUsernameOrPasswordError), 500)
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
		self.JsonResponseWithoutError(http, gin.H{})
		return
	}
	if params.GroupName == logic.SettingGroupUser {
		self.JsonResponseWithoutError(http, row.Value)
		return
	}
	refValue := reflect.ValueOf(row.Value).Elem()
	name := strings.ToUpper(string(params.Name[0])) + params.Name[1:]
	if refValue.FieldByName(name).IsValid() {
		self.JsonResponseWithoutError(http, refValue.FieldByName(name).Interface())
	} else {
		self.JsonResponseWithoutError(http, gin.H{})
	}
	return
}

func (self Setting) SaveConfig(http *gin.Context) {
	type ParamsValidate struct {
		Theme        *accessor.ThemeConfig        `json:"theme"`
		Console      *accessor.ThemeConsoleConfig `json:"console"`
		Notification *accessor.Notification       `json:"notification"`
		Login        *accessor.Login              `json:"login"`
		TwoFa        *accessor.TwoFa              `json:"twoFa"`
		SaveCache    bool                         `json:"saveCache"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	var value interface{}
	settingRow := &entity.Setting{}

	if params.Theme != nil {
		settingRow = &entity.Setting{
			GroupName: logic.SettingGroupSetting,
			Name:      logic.SettingGroupSettingThemeConfig,
			Value: &accessor.SettingValueOption{
				ThemeConfig: params.Theme,
			},
		}
		value = params.Theme
	}

	if params.Console != nil {
		settingRow = &entity.Setting{
			GroupName: logic.SettingGroupSetting,
			Name:      logic.SettingGroupSettingThemeConsoleConfig,
			Value: &accessor.SettingValueOption{
				ThemeConsoleConfig: params.Console,
			},
		}
		value = params.Console
	}

	if params.Notification != nil {
		settingRow = &entity.Setting{
			GroupName: logic.SettingGroupSetting,
			Name:      logic.SettingGroupSettingNotification,
			Value: &accessor.SettingValueOption{
				Notification: params.Notification,
			},
		}
		value = params.Notification
	}

	if params.Login != nil {
		settingRow = &entity.Setting{
			GroupName: logic.SettingGroupSetting,
			Name:      logic.SettingGroupSettingLogin,
			Value: &accessor.SettingValueOption{
				Login: params.Login,
			},
		}
		// 清空登录缓存
		for key, _ := range storage.Cache.Items() {
			if strings.HasPrefix(key, "login:failed") {
				storage.Cache.Delete(key)
			}
		}
		value = params.Login
	}

	err := logic.Setting{}.Save(settingRow)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if params.SaveCache && settingRow != nil {
		storage.Cache.Set(fmt.Sprintf(storage.CacheKeySetting, settingRow.Name), value, cache.DefaultExpiration)
	}

	facade.GetEvent().Publish(event.SettingSaveEvent, event.SettingPayload{
		GroupName: settingRow.GroupName,
		Name:      settingRow.Name,
	})

	self.JsonSuccessResponse(http)
	return
}

func (self Setting) Delete(http *gin.Context) {
	type ParamsValidate struct {
		GroupName string `json:"groupName" binding:"required"`
		Name      string `json:"name" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	err := logic.Setting{}.Delete(params.GroupName, params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Home) NotificationEmailTest(http *gin.Context) {
	type ParamsValidate struct {
		EmailServer *accessor.NotificationEmailServer `json:"emailServer" binding:"required"`
		Subject     string                            `json:"subject" binding:"required"`
		Content     string                            `json:"content" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	err := logic.Notice{}.Send(params.EmailServer, params.EmailServer.Email, params.Subject, params.Content)
	if err != nil {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageSettingBasicEmailInvalid, err.Error()), 500)
		return
	}
	self.JsonSuccessResponse(http)
}

func (self Home) Notification(http *gin.Context) {
	type ParamsValidate struct {
		Channel string `json:"channel"`
		Subject string `json:"subject"`
		Content string `json:"content" binding:"required"`
		Target  string `json:"target" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	setting := accessor.Notification{}
	logic.Setting{}.GetByKey(logic.SettingGroupSetting, logic.SettingGroupSettingNotification, &setting)

	if params.Subject == "" {
		params.Subject = "DPanel Notification"
	}

	if params.Channel == "email" {
		err := logic.Notice{}.Send(setting.EmailServer, params.Target, params.Subject, params.Content)
		if err != nil {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageSettingBasicEmailInvalid, err.Error()), 500)
			return
		}
	}
	self.JsonSuccessResponse(http)
}

func (self Home) Cache(http *gin.Context) {
	type ParamsValidate struct {
		Key   string `json:"key" binding:"required"`
		Value string `json:"value"`
		Keep  int    `json:"keep"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	cacheKey := fmt.Sprintf(storage.CacheKeyConsoleData, params.Key)
	if params.Value == "" {
		v, ok := storage.Cache.Get(cacheKey)
		self.JsonResponseWithoutError(http, gin.H{
			"value": v,
			"found": ok,
		})
		return
	}

	exp := cache.DefaultExpiration
	if params.Keep > -1 {
		exp = time.Duration(params.Keep) * time.Second
	}
	storage.Cache.Set(cacheKey, params.Value, exp)

	self.JsonResponseWithoutError(http, gin.H{
		"value": params.Value,
		"found": true,
	})
	return
}
