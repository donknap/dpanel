package common

import (
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/middleware"
	"net/http"
	"strings"
)

type CorsMiddleware struct {
	middleware.Abstract
}

func (self CorsMiddleware) Process(ctx *gin.Context) {
	if host, ok := self.isAllow(ctx); ok {
		ctx.Header("Access-Control-Allow-Origin", host)
		ctx.Header("Access-Control-Allow-Headers", "Content-Type,AccessToken,X-CSRF-Token, Authorization")
		ctx.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		ctx.Header("Access-Control-Expose-Headers", self.getAllowHeader())
		ctx.Header("Access-Control-Allow-Credentials", "true")
		if ctx.Request.Method == "OPTIONS" {
			ctx.AbortWithStatus(http.StatusNoContent)
		}
	}
	ctx.Next()
}

func (self CorsMiddleware) isAllow(ctx *gin.Context) (string, bool) {
	host := ctx.Request.Header.Get("origin")
	if host == "" {
		host = ctx.Request.Header.Get("referer")
	}
	if host == "" {
		return "", false
	}
	allowUrl := facade.GetConfig().GetStringSlice("app.cors")
	for _, value := range allowUrl {
		if value == host {
			return host, true
		}
	}
	return "", false
}

func (self CorsMiddleware) getAllowHeader() string {
	allowHeader := []string{
		"Content-Length",
		"Content-Type",
		"X-Auth-Token",
		"Origin",
		"Authorization",
		"X-Requested-With",
		"x-requested-with",
		"x-xsrf-token",
		"x-csrf-token",
		"x-w7-from",
		"access-token",
		"Api-Version",
		"Access-Control-Allow-Origin",
		"Access-Control-Allow-Headers",
		"Access-Control-Allow-Methods",
		"authority",
		"uid",
		"uuid",
	}
	return strings.Join(allowHeader, ",")
}
