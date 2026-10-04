package common

import (
	"errors"
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
	ErrLogin          = function.ErrorMessage(define.ErrorMessageUserLogin)
	anonymousApiPaths = []string{
		"/common/user/login",
		"/common/user/create-founder",
		"/common/user/login-info",
		"/common/user/oauth/callback",
		"/common/user/oauth/providers",
		"/common/user/oauth/authorize",
		"/pro/home/login-info",
		"/pro/user/reset-info",
		"/pro/passkey/auth",
		"/pro/passkey/verify-code",
	}
	founderApiPaths = []string{
		"/pro/passkey/save-setting",
		"/pro/passkey/prepare",
		"/pro/passkey/create",
		"/pro/passkey/get-list",
		"/pro/passkey/delete",
	}
)

func (self AuthMiddleware) Process(http *gin.Context) {
	currentUrlPath := http.Request.URL.Path
	apiPath := strings.TrimPrefix(currentUrlPath, function.RouterRootApi())
	if function.InArray(anonymousApiPaths, apiPath) ||
		(!strings.HasPrefix(currentUrlPath, function.RouterRootApi()) && !strings.HasPrefix(currentUrlPath, function.RouterRootWs())) {
		http.Next()
		return
	}

	authToken := http.GetHeader("X-DPanel-Authorization")
	if authToken == "" {
		authToken = http.GetHeader("Authorization")
	}
	if authToken == "" {
		if queryToken := http.Query("token"); queryToken != "" {
			authToken = "Bearer " + queryToken
		}
	}

	if authToken == "" {
		self.JsonResponseWithError(http, ErrLogin, 401)
		http.AbortWithStatus(401)
		return
	}
	authLog := slog.With("url", currentUrlPath, "peerAddr", http.Request.RemoteAddr, "reportedClientIP", http.ClientIP())
	authCode := strings.Split(authToken, "Bearer ")
	if len(authCode) != 2 {
		authLog.Debug("auth middleware", "reason", "authorization is not in Bearer format")
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
		reason := "JWT could not be parsed"
		switch {
		case errors.Is(err, jwt.ErrTokenMalformed):
			reason = "JWT is malformed"
		case errors.Is(err, jwt.ErrTokenSignatureInvalid):
			reason = "JWT signature is invalid"
		case errors.Is(err, jwt.ErrTokenExpired):
			reason = "JWT has expired"
		case errors.Is(err, jwt.ErrTokenNotValidYet):
			reason = "JWT is not valid yet"
		}
		authLog.Debug("auth middleware", "reason", reason)
		self.JsonResponseWithError(http, ErrLogin, 401)
		http.AbortWithStatus(401)
		return
	}

	if token.Valid {
		issuedAt, err := token.Claims.GetIssuedAt()
		if err != nil {
			authLog.Debug("auth middleware", "reason", "JWT issued-at claim is invalid")
			self.JsonResponseWithError(http, ErrLogin, 401)
			http.AbortWithStatus(401)
			return
		}

		// Jwt 签发时间必须大于服务启动时间一致，如果签发时间小于启动时间则表示服务重启过，Jwt 全部失效
		if v, ok := storage.Cache.Get(storage.CacheKeyCommonServerStartTime); !ok || issuedAt == nil || issuedAt.Before(v.(time.Time)) {
			authLog.Debug("auth middleware", "reason", "JWT was issued before server start", "issuedAt", issuedAt, "serverStartedAt", v)
			self.JsonResponseWithError(http, ErrLogin, 401)
			http.AbortWithStatus(401)
			return
		}

		currentUser, err := new(logic.Setting).GetValueById(myUserInfo.UserId)
		if err != nil || currentUser.Value == nil || currentUser.GroupName != logic.SettingGroupUser || currentUser.Value.UserStatus == logic.SettingGroupUserStatusDisable ||
			myUserInfo.ID != (logic.User{}).GetTokenId(currentUser) {
			self.JsonResponseWithError(http, ErrLogin, 401)
			http.AbortWithStatus(401)
			return
		}

		authenticated := myUserInfo.AutoLogin
		if !authenticated {
			if v, ok := storage.Cache.Get(fmt.Sprintf(storage.CacheKeyCommonUserInfo, myUserInfo.UserId)); ok {
				_, authenticated = v.(logic.UserInfo)
			}
		}
		if authenticated {
			if function.InArray(founderApiPaths, apiPath) && currentUser.Name != logic.SettingGroupUserFounder {
				self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageUserNoPermission), 403)
				http.AbortWithStatus(403)
				return
			}
			myUserInfo.Fd = http.GetHeader("AuthorizationFd")
			http.Set("userInfo", myUserInfo)
			http.Next()
			return
		}
		authLog.Debug("auth middleware", "reason", "user session was not found", "userId", myUserInfo.UserId)
		self.JsonResponseWithError(http, ErrLogin, 401)
		http.AbortWithStatus(401)
		return
	}
	authLog.Debug("auth middleware", "reason", "JWT is invalid")
	self.JsonResponseWithError(http, ErrLogin, 401)
	http.AbortWithStatus(401)
	return
}
