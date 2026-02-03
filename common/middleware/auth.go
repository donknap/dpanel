package common

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/middleware"
)

type AuthMiddleware struct {
	middleware.Abstract
}

var (
	ErrLogin = function.ErrorMessage(define.ErrorMessageUserLogin)
)

func (self AuthMiddleware) Process(http *gin.Context) {
	currentUrlPath := http.Request.URL.Path
	if strings.Contains(currentUrlPath, "/common/user/login") ||
		strings.Contains(currentUrlPath, "/common/user/create-founder") ||
		strings.Contains(currentUrlPath, "/common/user/oauth/callback") ||
		strings.Contains(currentUrlPath, "/pro/home/login-info") ||
		strings.Contains(currentUrlPath, "/pro/user/reset-info") ||
		(!strings.HasPrefix(currentUrlPath, function.RouterRootApi()) && !strings.HasPrefix(currentUrlPath, function.RouterRootWs())) {
		http.Next()
		return
	}

	var authToken = ""
	authToken = http.GetHeader("Authorization")
	if authToken == "" {
		authToken = "Bearer " + http.Query("token")
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
	token, err := jwt.ParseWithClaims(authCode[1], &myUserInfo, func(t *jwt.Token) (interface{}, error) {
		var rsaKeyContent []byte
		if v, ok := storage.Cache.Get(storage.CacheKeyRsaKey); ok {
			rsaKeyContent = v.([]byte)
		}
		privateKey, err := function.RSAParsePrivateKey(rsaKeyContent)
		if err != nil {
			return nil, err
		}
		return &privateKey.PublicKey, nil
	}, jwt.WithValidMethods([]string{"RS512"}))
	if err != nil {
		slog.Debug("auth middleware", "url", currentUrlPath, "code", authCode)
		self.JsonResponseWithError(http, ErrLogin, 401)
		http.AbortWithStatus(401)
		return
	}

	if token.Valid {
		issuedAt, err := token.Claims.GetIssuedAt()
		if err != nil {
			slog.Debug("auth middleware", "error", "no issuedAt time", "jwt", authToken)
			self.JsonResponseWithError(http, ErrLogin, 401)
			http.AbortWithStatus(401)
			return
		}

		// Jwt 签发时间必须大于服务启动时间一致，如果签发时间小于启动时间则表示服务重启过，Jwt 全部失效
		if v, ok := storage.Cache.Get(storage.CacheKeyCommonServerStartTime); !ok || issuedAt == nil || issuedAt.Before(v.(time.Time)) {
			slog.Debug("auth middleware", "error", "issuedAt time before server start time", "issuedAt", issuedAt, "serverStartedAt", v.(time.Time))
			self.JsonResponseWithError(http, ErrLogin, 401)
			http.AbortWithStatus(401)
			return
		}

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
		slog.Debug("auth middleware", "err", "user not found", "userInfo", myUserInfo)
		self.JsonResponseWithError(http, ErrLogin, 401)
		http.AbortWithStatus(401)
		return
	}
	slog.Debug("auth middleware", "url", currentUrlPath, "code", authCode)
	self.JsonResponseWithError(http, ErrLogin, 401)
	http.AbortWithStatus(401)
	return
}
