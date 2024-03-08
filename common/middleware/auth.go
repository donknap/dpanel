package common

import (
	"errors"
	"fmt"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/function"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/we7coreteam/w7-rangine-go/src/http/middleware"
	"strings"
)

type AuthMiddleware struct {
	middleware.Abstract
}

func (self AuthMiddleware) Process(http *gin.Context) {
	if function.InArray([]string{
		"/home/ws/notice",
		"/common/user/login",
	}, http.Request.URL.Path) {
		http.Next()
		return
	}
	if http.GetHeader("Authorization") == "" {
		self.JsonResponseWithError(http, errors.New("请先登录"), 401)
		http.AbortWithStatus(401)
		return
	}
	authCode := strings.Split(http.GetHeader("Authorization"), "Bearer ")

	if len(authCode) != 2 {
		self.JsonResponseWithError(http, errors.New("请先登录"), 401)
		http.AbortWithStatus(401)
		return
	}

	jwtSecret := logic.User{}.GetJwtSecret()
	myUserInfo := logic.UserToken{}
	token, err := jwt.ParseWithClaims(authCode[1], &myUserInfo, func(t *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		self.JsonResponseWithError(http, err, 401)
		http.AbortWithStatus(401)
		return
	}
	if token.Valid {
		fmt.Printf("%v \n", myUserInfo)
		http.Next()
		return
	}
	self.JsonResponseWithError(http, errors.New("请先登录"), 401)
	http.AbortWithStatus(401)
	return
}
