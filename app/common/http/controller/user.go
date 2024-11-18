package controller

import (
	"errors"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"time"
)

type User struct {
	controller.Abstract
}

func (self User) Login(http *gin.Context) {
	type ParamsValidate struct {
		Username        string `json:"username" binding:"required"`
		Password        string `json:"password" binding:"required"`
		ConfirmPassword string `json:"confirmPassword"`
		AutoLogin       bool   `json:"autoLogin"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if params.ConfirmPassword != "" {
		founder, _ := logic.Setting{}.GetValue(logic.SettingGroupUser, logic.SettingGroupUserFounder)
		if founder != nil {
			self.JsonResponseWithError(http, errors.New("管理员配置已经存在，无法初始化。请去系统设置中修改"), 500)
			return
		}
		if params.Password != params.ConfirmPassword {
			self.JsonResponseWithError(http, errors.New("两次输入密码不一致"), 500)
			return
		}
		err := dao.Setting.Create(&entity.Setting{
			GroupName: logic.SettingGroupUser,
			Name:      logic.SettingGroupUserFounder,
			Value: &accessor.SettingValueOption{
				Password: logic.User{}.GetMd5Password(params.Password, params.Username),
				Username: params.Username,
			},
		})
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	var expireAddTime time.Duration
	if params.AutoLogin {
		expireAddTime = time.Hour * 24 * 30
	} else {
		expireAddTime = time.Hour * 24
	}

	currentUser, err := logic.Setting{}.GetValue(logic.SettingGroupUser, logic.SettingGroupUserFounder)
	if err != nil {
		self.JsonResponseWithError(http, errors.New("创始人配置不存在，请重新安装"+err.Error()), 500)
		return
	}
	password := logic.User{}.GetMd5Password(params.Password, params.Username)
	if params.Username == currentUser.Value.Username && currentUser.Value.Password == password {
		jwtSecret := logic.User{}.GetJwtSecret()
		jwt := jwt.NewWithClaims(jwt.SigningMethodHS256, logic.UserInfo{
			UserId:       currentUser.ID,
			Username:     currentUser.Value.Username,
			RoleIdentity: currentUser.Name,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(expireAddTime)),
			},
		})
		code, err := jwt.SignedString(jwtSecret)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		self.JsonResponseWithoutError(http, gin.H{
			"accessToken": code,
		})
		return
	} else {
		self.JsonResponseWithError(http, errors.New("用户名密码错误"), 500)
		return
	}
}

func (self User) GetUserInfo(http *gin.Context) {
	data, exists := http.Get("userInfo")
	if !exists {
		self.JsonResponseWithError(http, errors.New("请先登录"), 401)
		http.AbortWithStatus(401)
		return
	}
	userInfo := data.(logic.UserInfo)

	feature := struct {
		ComposeStore bool `json:"composeStore"`
	}{
		ComposeStore: false,
	}
	count, _ := dao.Store.Count()
	if count > 0 {
		feature.ComposeStore = true
	}

	self.JsonResponseWithoutError(http, gin.H{
		"user":    userInfo,
		"feature": feature,
	})
	return
}
