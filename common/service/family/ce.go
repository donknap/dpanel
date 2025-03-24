//go:build !pe && !ee

package family

import (
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/types"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/console"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/server"
	"log/slog"
)

type Provider struct {
}

func (self Provider) Register(httpServer *server.Server, consoleServer console.Console) {
	slog.Debug("provider load community edition")
	httpServer.RegisterRouters(func(engine *gin.Engine) {
		engine.POST("/api/pro/*path", notSupportedApi)
	})
}

func (self Provider) Feature() []string {
	return []string{
		types.FeatureFamilyCe,
	}
}

func (self Provider) Check(name string) bool {
	return function.InArray(self.Feature(), name)
}
