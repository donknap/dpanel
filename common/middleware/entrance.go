package common

import (
	"bytes"
	"crypto/hmac"
	"encoding/base64"
	"html/template"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

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
		self.renderUnavailable(httpContext)
		return
	}

	startTime, ok := storage.LoadCache[time.Time](storage.CacheKeyCommonServerStartTime)
	if !ok {
		self.renderUnavailable(httpContext)
		return
	}
	cookieValue := function.HmacSha256([]byte(strconv.FormatInt(startTime.UnixNano(), 10)), []byte(entrance))
	if cookie, err := httpContext.Request.Cookie(EntranceCookieName); err == nil && hmac.Equal([]byte(cookie.Value), []byte(cookieValue)) {
		httpContext.Next()
		return
	}

	entrancePath := strings.TrimRight(function.RouterUri("/"+entrance), "/")
	if requestPath != entrancePath {
		http.Redirect(httpContext.Writer, httpContext.Request, function.RouterUri("/"), http.StatusFound)
		httpContext.Abort()
		return
	}
	httpContext.SetCookie(EntranceCookieName, cookieValue, 0, "/", "", false, true)
	httpContext.Next()
}

func (self EntranceMiddleware) renderUnavailable(httpContext *gin.Context) {
	if value, ok := storage.Cache.Get(storage.CacheKeyAsset); ok {
		if asset, ok := value.(fs.FS); ok {
			if content, err := fs.ReadFile(asset, "asset/security-entrance.html"); err == nil {
				logoData := ""
				darkLogoData := ""
				if logo, err := fs.ReadFile(asset, "asset/static/img/logo.png"); err == nil {
					logoData = base64.StdEncoding.EncodeToString(logo)
				}
				if logo, err := fs.ReadFile(asset, "asset/static/img/logo-dark.png"); err == nil {
					darkLogoData = base64.StdEncoding.EncodeToString(logo)
				}
				pageData := struct {
					LogoData     string
					DarkLogoData string
				}{LogoData: logoData, DarkLogoData: darkLogoData}

				page, err := template.New("security-entrance").Parse(string(content))
				if err == nil {
					var rendered bytes.Buffer
					err = page.Execute(&rendered, pageData)
					if err == nil {
						httpContext.Data(http.StatusOK, "text/html; charset=utf-8", rendered.Bytes())
						httpContext.Abort()
						return
					}
				}
			}
		}
	}

	http.NotFound(httpContext.Writer, httpContext.Request)
	httpContext.Abort()
}

func (self EntranceMiddleware) GetSecurityEntrance() string {
	var entrance *string

	if cached, ok := storage.LoadCache[*accessor.SettingValueOption](storage.CacheKeySettingLogin); ok && cached != nil && cached.Login != nil {
		entrance = cached.Login.Entrance
	}

	if entrance == nil {
		login := accessor.Login{}
		if ok := (logic.Setting{}).GetByKey(logic.SettingGroupSetting, logic.SettingGroupSettingLogin, &login); ok && login.Entrance != nil {
			entrance = login.Entrance
			storage.Cache.Set(storage.CacheKeySettingLogin, &accessor.SettingValueOption{Login: &login}, cache.NoExpiration)
		}
	}

	if entrance == nil {
		entrance = function.Ptr[string](facade.GetConfig().GetString("system.entrance"))
	}

	return strings.Trim(*entrance, "/")
}
