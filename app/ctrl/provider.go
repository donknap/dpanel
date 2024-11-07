package ctrl

import (
	"github.com/donknap/dpanel/app/ctrl/command"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/console"
)

type Provider struct {
}

func (provider *Provider) Register(console console.Console) {
	console.RegisterCommand(new(command.UserReset))
}
