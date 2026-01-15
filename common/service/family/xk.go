//go:build xk

package family

import (
	"log/slog"

	"github.com/donknap/dpanel/app/pro/xk"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/server"
)

type Provider struct {
}

func (self Provider) Register(httpServer *server.Server) {
	slog.Debug("provider load xk edition")
	new(xk.Provider).Register(httpServer)
}

func (self Provider) Feature() []string {
	return new(xk.Provider).Feature()
}

func (self Provider) Middleware() []gin.HandlerFunc {
	return nil
}
