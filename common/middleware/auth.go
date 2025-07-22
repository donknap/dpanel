package common

import (
	"fmt"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
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
	ErrLogin = function.ErrorMessage(define.ErrorMessageUserLogin)
)

func (self AuthMiddleware) Process(http *gin.Context) {
	currentUrlPath := http.Request.URL.Path
	if strings.Contains(currentUrlPath, "/api/common/user/login") ||
		strings.Contains(currentUrlPath, "/pro/home/login-info") ||
		strings.Contains(currentUrlPath, "/api/common/user/create-founder") ||
		strings.Contains(currentUrlPath, "/pro/user/reset-info") ||
		strings.Contains(currentUrlPath, "/xk/user/oauth/callback") ||
		(!strings.HasPrefix(currentUrlPath, "/api") && !strings.HasPrefix(currentUrlPath, "/ws")) {
		http.Next()
		return
	}

	var authToken = ""
	if strings.HasPrefix(currentUrlPath, "/ws/") {
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
		slog.Debug("auth middleware", "url", currentUrlPath, "code", authCode)
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
		slog.Debug("auth middleware", "url", currentUrlPath, "code", authCode)
		self.JsonResponseWithError(http, ErrLogin, 401)
		http.AbortWithStatus(401)
		return
	}
	if token.Valid {
		if myUserInfo.AutoLogin {
			if _, err := new(logic.Setting).GetValueById(myUserInfo.UserId); err == nil {
				myUserInfo.Fd = http.GetHeader("AuthorizationFd")
				http.Set("userInfo", myUserInfo)
				http.Next()
				return
			}
		} else {
			if v, ok := storage.Cache.Get(fmt.Sprintf(storage.CacheKeyCommonUserInfo, myUserInfo.UserId)); ok {
				if _, ok := v.(logic.UserInfo); ok {
					myUserInfo.Fd = http.GetHeader("AuthorizationFd")
					http.Set("userInfo", myUserInfo)
					http.Next()
					return
				}
			}
		}
		slog.Debug("auth get cache user error", "url", currentUrlPath, "jwt", authToken, "userInfo", myUserInfo)
		self.JsonResponseWithError(http, ErrLogin, 401)
		http.AbortWithStatus(401)
		return
	}
	slog.Debug("auth middleware", "url", currentUrlPath, "code", authCode)
	self.JsonResponseWithError(http, ErrLogin, 401)
	http.AbortWithStatus(401)
	return
}
