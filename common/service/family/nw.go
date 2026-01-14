//go:build nw

package family

/**
 * ASUS_NW
 */

import (
	"log/slog"

	"github.com/donknap/dpanel/app/pro/nw"
	"github.com/donknap/dpanel/common/function"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/server"
)

type Provider struct {
}

func (self *Provider) Register(httpServer *server.Server) {
	slog.Debug("provider load ASUS_NW edition")
	new(nw.Provider).Register()
}

func (self Provider) Feature() []string {
	return new(nw.Provider).Feature()
}

func (self Provider) Check(name string) bool {
	return function.InArray(self.Feature(), name)
}

func (self Provider) Middleware() []gin.HandlerFunc {
	return nil
}
