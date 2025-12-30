package common

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/middleware"
)

type CacheMiddleware struct {
	middleware.Abstract
}

func (self CacheMiddleware) Process(http *gin.Context) {
	url := http.Request.URL.Path
	if strings.HasPrefix(url, "/dpanel/static") || strings.HasPrefix(url, "/favicon.ico") {
		http.Header("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate, max-age=0")
		http.Header("Pragma", "no-cache")
		http.Header("Expires", "0")
	}
	http.Next()
	return
}
