package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go-support/src/facade"
	"github.com/we7coreteam/w7-rangine-go/src/http/middleware"
	"time"
)

type Home struct {
	middleware.Abstract
}

func (home Home) Process(ctx *gin.Context) {
	log, _ := facade.GetLoggerFactory().Channel("default")
	log.Info("route middleware test, req time: " + time.Now().String())

	ctx.Next()
}
