//go:build ee

package family

import (
	"github.com/donknap/dpanel/app/pro/ee"
	"github.com/donknap/dpanel/common/function"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/server"
	"log/slog"
)

type Provider struct {
}

func (self *Provider) Register(httpServer *server.Server) {
	slog.Debug("provider load enterprise edition")
	new(ee.Provider).Register(httpServer)
}

func (self Provider) Feature() []string {
	return new(ee.Provider).Feature()
}

func (self Provider) Check(name string) bool {
	return function.InArray(self.Feature(), name)
}
