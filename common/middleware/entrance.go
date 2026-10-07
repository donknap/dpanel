package common

import (
	"bytes"
	"crypto/hmac"
	"encoding/base64"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/middleware"
)

const EntranceCookieName = "DPanelEntrance"

const entranceRenewCacheKey = "entrance:renew:"
const entranceCookieMaxAge = 7 * 24 * 60 * 60
const entranceRenewInterval = 8 * time.Hour

type EntranceMiddleware struct {
	middleware.Abstract
}

func (self EntranceMiddleware) Process(httpContext *gin.Context) {
	userInfo, loggedIn := httpContext.Get("userInfo")
	renewKey := ""
	if loggedIn {
		user := userInfo.(logic.UserInfo)
		renewKey = fmt.Sprintf("%s%d:%d", entranceRenewCacheKey, user.UserId, user.IssuedAt.Unix())
		if _, locked := storage.Cache.Get(renewKey); locked {
			httpContext.Next()
			return
		}
	}

	login := logic.Setting{}.GetLoginSetting()
	if login.SystemEntrance == nil || !login.SystemEntrance.Enable {
		httpContext.Next()
		return
	}
	entrance := login.SystemEntrance.Config
	if login.SystemEntrance.Entrance != nil {
		entrance = *login.SystemEntrance.Entrance
	}
	if entrance == "" {
		httpContext.Next()
		return
	}

	entrancePath := strings.TrimRight(function.RouterUri("/"+entrance), "/")
	requestPath := strings.TrimRight(httpContext.Request.URL.Path, "/")
	if loggedIn {
		requestPath = entrancePath
	}
	rootPath := strings.TrimRight(function.RouterUri("/"), "/")
	if rootPath == "" {
		rootPath = "/"
	}
	if requestPath == "" {
		requestPath = "/"
	}
	startTime, ok := storage.LoadCache[time.Time](storage.CacheKeyCommonServerStartTime)
	if !ok {
		self.renderUnavailable(httpContext)
		return
	}
	cookieValue := function.HmacSha256([]byte(strconv.FormatInt(startTime.UnixNano(), 10)), []byte(entrance))
	if cookie, err := httpContext.Request.Cookie(EntranceCookieName); !loggedIn && err == nil && hmac.Equal([]byte(cookie.Value), []byte(cookieValue)) {
		httpContext.Next()
		return
	}
	if requestPath == rootPath && !loggedIn {
		self.renderUnavailable(httpContext)
		return
	}
	if requestPath != entrancePath {
		http.Redirect(httpContext.Writer, httpContext.Request, function.RouterUri("/"), http.StatusFound)
		httpContext.Abort()
		return
	}
	httpContext.SetCookie(EntranceCookieName, cookieValue, entranceCookieMaxAge, "/", "", false, true)
	if loggedIn {
		storage.Cache.Set(renewKey, true, entranceRenewInterval)
	}
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
					LogoData     string `json:"logoData"`
					DarkLogoData string `json:"darkLogoData"`
				}{LogoData: logoData, DarkLogoData: darkLogoData}

				page, err := template.New("security-entrance").Parse(string(content))
				if err == nil {
					var rendered bytes.Buffer
					err = page.Execute(&rendered, function.StructToMap(pageData))
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
