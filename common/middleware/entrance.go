package common

import (
	"net/http"
	"strings"

	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/gin-gonic/gin"
	"github.com/patrickmn/go-cache"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/middleware"
)

const EntranceCookieName = "DPanelEntrance"

type EntranceMiddleware struct {
	middleware.Abstract
}

func (self EntranceMiddleware) Process(httpContext *gin.Context) {
	if _, loggedIn := httpContext.Get("userInfo"); loggedIn {
		httpContext.Next()
		return
	}

	entrance := self.GetSecurityEntrance()
	if entrance == "" {
		httpContext.Next()
		return
	}

	requestPath := strings.TrimRight(httpContext.Request.URL.Path, "/")
	rootPath := strings.TrimRight(function.RouterUri("/"), "/")
	if rootPath == "" {
		rootPath = "/"
	}
	if requestPath == "" {
		requestPath = "/"
	}
	if requestPath == rootPath {
		http.NotFound(httpContext.Writer, httpContext.Request)
		httpContext.Abort()
		return
	}

	if cookie, err := httpContext.Request.Cookie(EntranceCookieName); err == nil && cookie.Value == entrance {
		httpContext.Next()
		return
	}

	entrancePath := strings.TrimRight(function.RouterUri("/"+entrance), "/")
	if requestPath != entrancePath {
		http.NotFound(httpContext.Writer, httpContext.Request)
		httpContext.Abort()
		return
	}
	httpContext.SetCookie(EntranceCookieName, entrance, 0, "/", "", false, true)
	httpContext.Next()
}

func (self EntranceMiddleware) GetSecurityEntrance() string {
	if cached, ok := storage.LoadCache[*accessor.SettingValueOption](storage.CacheKeySettingLogin); ok {
		if cached == nil || cached.Login == nil {
			return ""
		}
		return strings.Trim(cached.Login.Entrance, "/")
	}

	login := accessor.Login{}
	if (logic.Setting{}).GetByKey(logic.SettingGroupSetting, logic.SettingGroupSettingLogin, &login) {
		login.Entrance = strings.Trim(login.Entrance, "/")
		storage.Cache.Set(storage.CacheKeySettingLogin, &accessor.SettingValueOption{Login: &login}, cache.NoExpiration)
		return login.Entrance
	}

	entrance := strings.Trim(facade.GetConfig().GetString("system.entrance"), "/")
	storage.Cache.Set(storage.CacheKeySettingLogin, &accessor.SettingValueOption{Login: &accessor.Login{Entrance: entrance}}, cache.NoExpiration)
	return entrance
}
