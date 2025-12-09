package common

import (
	"log/slog"
	"runtime"

	"github.com/donknap/dpanel/common/service/ws"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/middleware"
)

type DebugMiddleware struct {
	middleware.Abstract
}

func (self DebugMiddleware) Process(ctx *gin.Context) {
	slog.Info("runtime info", "url", ctx.Request.URL, "goroutine", runtime.NumGoroutine(), "client total", ws.GetCollect().Total(), "progress total", ws.GetCollect().ProgressTotal())
	ctx.Next()
}
