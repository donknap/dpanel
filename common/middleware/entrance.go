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
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/gin-gonic/gin"
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
