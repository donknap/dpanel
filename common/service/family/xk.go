//go:build xk

package family

import (
	"github.com/donknap/dpanel/app/pro/xk"
	"github.com/donknap/dpanel/common/function"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/server"
	"log/slog"
)

type Provider struct {
}

func (self *Provider) Register(httpServer *server.Server) {
	slog.Debug("provider load xk edition")
	new(xk.Provider).Register(httpServer)
}

func (self Provider) Feature() []string {
	return new(xk.Provider).Feature()
}

func (self Provider) Check(name string) bool {
	return function.InArray(self.Feature(), name)
}
