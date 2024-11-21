package common

import (
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/middleware"
	http2 "net/http"
	"strconv"
	"strings"
	"time"
)

type CacheMiddleware struct {
	middleware.Abstract
}

func (self CacheMiddleware) Process(http *gin.Context) {
	if strings.HasPrefix(http.Request.URL.Path, "/dpanel/static") {
		defaultMaxAge := 604800
		http.Writer.Header().Add("Cache-Control", "public, max-age="+strconv.Itoa(defaultMaxAge))
		http.Writer.Header().Add("Expires", time.Now().Add(time.Duration(defaultMaxAge)*time.Second).UTC().Format(http2.TimeFormat))
		http.Writer.WriteHeader(304)
	}
	http.Next()
	return
}
