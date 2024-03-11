package controller

import (
	"errors"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
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
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	var expireAddTime time.Duration
	if params.AutoLogin {
		expireAddTime = time.Hour * 24 * 30
	} else {
		expireAddTime = time.Hour * 24
	}

	currentUser, err := logic.Setting{}.GetValue(logic.SettingUser, logic.SettingUserFounder)
	if err != nil {
		self.JsonResponseWithError(http, errors.New("创始人配置不存在，请重新安装"), 500)
		return
	}
	password := logic.User{}.GetMd5Password(params.Password, params.Username)
	if params.Username == (*currentUser.Value)["username"] && (*currentUser.Value)["password"] == password {
		jwtSecret := logic.User{}.GetJwtSecret()
		jwt := jwt.NewWithClaims(jwt.SigningMethodHS256, logic.UserInfo{
			UserId:       currentUser.ID,
			Username:     (*currentUser.Value)["username"],
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
	self.JsonResponseWithoutError(http, gin.H{
		"user": userInfo,
	})
	return
}
