//go:build pe

package provider

import (
	"github.com/donknap/dpanel/app/application"
	"github.com/donknap/dpanel/app/common"
	"github.com/donknap/dpanel/app/ctrl"
	"github.com/donknap/dpanel/app/pro"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/console"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/server"
	"log/slog"
)

type Provider struct {
}

func (providder *Provider) Register(httpServer *server.Server, consoleServer console.Console) {
	slog.Debug("provider load professional edition")

	new(common.Provider).Register(httpServer)

	new(application.Provider).Register(httpServer)
	new(ctrl.Provider).Register(consoleServer)
	new(pro.Provider).Register(httpServer)
}
