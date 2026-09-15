package common

import (
	"crypto/hmac"
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
	if currentUrlPath == function.RouterApiUri("/common/user/login") ||
		currentUrlPath == function.RouterApiUri("/common/user/login-info") ||
		currentUrlPath == function.RouterApiUri("/common/user/create-founder") ||
		currentUrlPath == function.RouterApiUri("/common/user/oauth/providers") ||
		currentUrlPath == function.RouterApiUri("/common/user/oauth/authorize") ||
		currentUrlPath == function.RouterApiUri("/common/user/oauth/callback") ||
		currentUrlPath == function.RouterApiUri("/pro/home/login-info") ||
		currentUrlPath == function.RouterApiUri("/pro/user/reset-info") ||
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
	authCode := strings.Split(authToken, "Bearer ")
	if len(authCode) != 2 {
		slog.Debug("auth middleware", "url", currentUrlPath, "error", "invalid authorization header")
		self.JsonResponseWithError(http, ErrLogin, 401)
		http.AbortWithStatus(401)
		return
	}

	myUserInfo := logic.UserInfo{}
	var rsaKeyContent []byte
	token, err := jwt.ParseWithClaims(authCode[1], &myUserInfo, func(t *jwt.Token) (interface{}, error) {
		if v, ok := storage.Cache.Get(storage.CacheKeyRsaKey); ok {
			rsaKeyContent = v.([]byte)
		}
		privateKey, err := function.RSAParsePrivateKey(rsaKeyContent)
		if err != nil {
			return nil, err
		}
		return &privateKey.PublicKey, nil
	}, jwt.WithValidMethods([]string{"RS512"}), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil {
		slog.Debug("auth middleware", "url", currentUrlPath, "error", "invalid token")
		self.JsonResponseWithError(http, ErrLogin, 401)
		http.AbortWithStatus(401)
		return
	}

	if token.Valid {
		issuedAt, err := token.Claims.GetIssuedAt()
		if err != nil {
			slog.Debug("auth middleware", "error", "no issuedAt time")
			self.JsonResponseWithError(http, ErrLogin, 401)
			http.AbortWithStatus(401)
			return
		}

		// Jwt 签发时间必须大于服务启动时间一致，如果签发时间小于启动时间则表示服务重启过，Jwt 全部失效
		if v, ok := storage.LoadCache[time.Time](storage.CacheKeyCommonServerStartTime); !ok || issuedAt == nil || issuedAt.Before(v) {
			slog.Debug("auth middleware", "error", "issuedAt time before server start time", "issuedAt", issuedAt, "serverStartedAt", v)
			self.JsonResponseWithError(http, ErrLogin, 401)
			http.AbortWithStatus(401)
			return
		}

		// 每次请求核对账户状态和凭据，改密、重置或停用后旧令牌立即失效。
		currentUser, err := new(logic.Setting).GetValueById(myUserInfo.UserId)
		if err != nil || currentUser.Value == nil || currentUser.GroupName != logic.SettingGroupUser ||
			currentUser.Value.UserStatus == logic.SettingGroupUserStatusDisable ||
			currentUser.Value.Username != myUserInfo.Username || currentUser.Name != myUserInfo.RoleIdentity ||
			myUserInfo.SessionVersion == "" || !hmac.Equal([]byte(myUserInfo.SessionVersion),
			[]byte((logic.User{}).SessionVersion(currentUser, rsaKeyContent))) {
			self.JsonResponseWithError(http, ErrLogin, 401)
			http.AbortWithStatus(401)
			return
		}

		if myUserInfo.AutoLogin {
			myUserInfo.Fd = http.GetHeader("AuthorizationFd")
			http.Set("userInfo", myUserInfo)
			http.Next()
			return
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
		slog.Debug("auth middleware", "err", "user not found", "userId", myUserInfo.UserId)
		self.JsonResponseWithError(http, ErrLogin, 401)
		http.AbortWithStatus(401)
		return
	}
	slog.Debug("auth middleware", "url", currentUrlPath, "error", "invalid token")
	self.JsonResponseWithError(http, ErrLogin, 401)
	http.AbortWithStatus(401)
	return
}
