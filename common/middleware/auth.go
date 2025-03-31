package common

import (
	"errors"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/middleware"
	"log/slog"
	"strings"
)

type AuthMiddleware struct {
	middleware.Abstract
}

var (
	ErrLogin = errors.New("请先登录")
)

func (self AuthMiddleware) Process(http *gin.Context) {
	if strings.Contains(http.Request.URL.Path, "/api/common/user/login") ||
		strings.Contains(http.Request.URL.Path, "/pro/home/login-info") ||
		strings.Contains(http.Request.URL.Path, "/api/common/user/create-founder") ||
		strings.Contains(http.Request.URL.Path, "/pro/user/reset-info") ||
		!strings.Contains(http.Request.URL.Path, "/api") {
		http.Next()
		return
	}

	var authToken = ""
	if strings.Contains(http.Request.URL.Path, "/common/ws") {
		authToken = "Bearer " + http.Query("token")
	} else {
		authToken = http.GetHeader("Authorization")
	}

	if authToken == "" {
		self.JsonResponseWithError(http, ErrLogin, 401)
		http.AbortWithStatus(401)
		return
	}
	authCode := strings.Split(authToken, "Bearer ")
	if len(authCode) != 2 {
		self.JsonResponseWithError(http, ErrLogin, 401)
		http.AbortWithStatus(401)
		return
	}

	myUserInfo := logic.UserInfo{}
	jwtSecret := logic.User{}.GetJwtSecret()
	token, err := jwt.ParseWithClaims(authCode[1], &myUserInfo, func(t *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		self.JsonResponseWithError(http, ErrLogin, 401)
		http.AbortWithStatus(401)
		return
	}
	if token.Valid {
		_, err := logic.Setting{}.GetValueById(myUserInfo.UserId)
		if err != nil {
			slog.Debug("auth get user", "error", err.Error())
			self.JsonResponseWithError(http, ErrLogin, 401)
			http.AbortWithStatus(401)
			return
		}
		myUserInfo.Fd = http.GetHeader("AuthorizationFd")
		http.Set("userInfo", myUserInfo)
		http.Next()
		return
	}
	self.JsonResponseWithError(http, ErrLogin, 401)
	http.AbortWithStatus(401)
	return
}
