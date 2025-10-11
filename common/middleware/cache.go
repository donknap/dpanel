package common

import (
	"strconv"
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
		defaultMaxAge := 3600
		http.Header("Cache-Control", "public, max-age="+strconv.Itoa(defaultMaxAge))
	}
	http.Next()
	return
}
