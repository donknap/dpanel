//go:build pe

package family

import (
	"github.com/donknap/dpanel/app/pro"
	"github.com/donknap/dpanel/common/function"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/console"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/server"
	"log/slog"
)

type Provider struct {
}

func (providder *Provider) Register(httpServer *server.Server, consoleServer console.Console) {
	slog.Debug("provider load professional edition")
	new(pro.Provider).Register(httpServer)
}

func (self Provider) Feature() []string {
	return []string{
		FeatureFamilyPe,
	}
}

func (self Provider) Check(name string) bool {
	return function.InArray(self.Feature(), name)
}
