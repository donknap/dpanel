package controller

import (
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/family"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/types"
	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"time"
)

type User struct {
	controller.Abstract
}

func (self User) Login(http *gin.Context) {
	type ParamsValidate struct {
		Username  string `json:"username" binding:"required"`
		Password  string `json:"password" binding:"required"`
		AutoLogin bool   `json:"autoLogin"`
		Code      string `json:"code"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	if !new(family.Provider).Check(types.FeatureFamilyCe) {
		twoFa := accessor.TwoFa{}
		exists := logic.Setting{}.GetByKey(logic.SettingGroupSetting, logic.SettingGroupSettingTwoFa, &twoFa)
		if exists && twoFa.Enable {
			if params.Code == "" {
				self.JsonResponseWithError(http, function.ErrorMessage(".userTwoFaEmpty"), 500)
				return
			}
			if !totp.Validate(params.Code, twoFa.Secret) {
				self.JsonResponseWithError(http, function.ErrorMessage(".userTwoFaNotCorrect"), 500)
				return
			}
		}
	}
	var code string
	var err error

	if err = new(logic.User).CheckLock(params.Username); err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	defer func() {
		logic.User{}.Lock(params.Username, code == "")
	}()

	currentUser, err := logic.User{}.GetUserByUsername(params.Username)
	if err != nil {
		self.JsonResponseWithError(http, notice.Message{}.New(".usernameOrPasswordError"), 500)
		return
	}
	if currentUser.Value.Password == "" {
		self.JsonResponseWithError(http, notice.Message{}.New(".usernameOrPasswordError"), 500)
		return
	}

	if currentUser.Value.UserStatus == logic.SettingGroupUserStatusDisable {
		self.JsonResponseWithError(http, notice.Message{}.New(".userDisable"), 500)
		return
	}

	if !(family.Provider{}).Check(types.FeatureFamilyEe) && currentUser.Name != logic.SettingGroupUserFounder {
		self.JsonResponseWithError(http, notice.Message{}.New(".userDisable"), 500)
		return
	}

	password := logic.User{}.GetMd5Password(params.Password, params.Username)
	if params.Username == currentUser.Value.Username && currentUser.Value.Password == password {
		code, err = logic.User{}.GetUserOauthToken(currentUser, params.AutoLogin)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		self.JsonResponseWithoutError(http, gin.H{
			"accessToken": code,
		})
		return
	} else {
		self.JsonResponseWithError(http, notice.Message{}.New(".usernameOrPasswordError"), 500)
		return
	}
}

func (self User) GetUserInfo(http *gin.Context) {
	result := gin.H{
		"menu":      make([]string, 0),
		"themeUser": make(map[string]string),
	}

	data, exists := http.Get("userInfo")
	if !exists {
		self.JsonResponseWithError(http, function.ErrorMessage(".login"), 401)
		http.AbortWithStatus(401)
		return
	}
	result["user"] = data.(logic.UserInfo)

	feature := make([]string, 0)

	if facade.GetConfig().GetString("app.env") == "lite" {
		feature = append(feature, types.FeatureVersionEnvLite)
	} else {
		feature = append(feature, types.FeatureVersionEnvStandard)
	}

	if docker.Sdk.Name == docker.DefaultClientName {
		feature = append(feature, types.FeatureDockerEnvLocal)
	}

	result["feature"] = append(feature, family.Provider{}.Feature()...)

	themeConfig := accessor.ThemeConfig{}
	logic.Setting{}.GetByKey(logic.SettingGroupSetting, logic.SettingGroupSettingThemeConfig, &themeConfig)
	result["theme"] = themeConfig

	self.JsonResponseWithoutError(http, result)
	return
}

func (self User) LoginInfo(http *gin.Context) {
	result := gin.H{
		"showRegister":  false,
		"showBuildName": true,
		"family":        facade.GetConfig().GetString("app.env"),
		"feature":       family.Provider{}.Feature(),
		"appName":       facade.GetConfig().GetString("app.name"),
	}
	_, err := logic.Setting{}.GetDPanelInfo()
	if err == nil {
		result["showBuildName"] = false
	}
	_, err = logic.Setting{}.GetValue(logic.SettingGroupUser, logic.SettingGroupUserFounder)
	if err != nil {
		result["showRegister"] = true
	}
	self.JsonResponseWithoutError(http, result)
	return
}

func (self User) CreateFounder(http *gin.Context) {
	type ParamsValidate struct {
		Username        string `json:"username" binding:"required"`
		Password        string `json:"password"`
		ConfirmPassword string `json:"confirmPassword"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	founder, _ := logic.Setting{}.GetValue(logic.SettingGroupUser, logic.SettingGroupUserFounder)
	if founder != nil {
		self.JsonResponseWithError(http, notice.Message{}.New(".userFounderExists"), 500)
		return
	}
	if params.Password != params.ConfirmPassword {
		self.JsonResponseWithError(http, notice.Message{}.New(".userPasswordConfirmFailed"), 500)
		return
	}

	if (logic.User{}.GetBuiltInPublicUsername()) == params.Username {
		self.JsonResponseWithServerError(http, notice.Message{}.New(".userFounderExists"))
		return
	}

	registerAt := time.Now()
	err := dao.Setting.Create(&entity.Setting{
		GroupName: logic.SettingGroupUser,
		Name:      logic.SettingGroupUserFounder,
		Value: &accessor.SettingValueOption{
			Password:   logic.User{}.GetMd5Password(params.Password, params.Username),
			Username:   params.Username,
			UserStatus: logic.SettingGroupUserStatusEnable,
			RegisterAt: &registerAt,
		},
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}
