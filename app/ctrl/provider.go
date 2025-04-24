package ctrl

import (
	"github.com/donknap/dpanel/app/ctrl/command/compose"
	"github.com/donknap/dpanel/app/ctrl/command/container"
	"github.com/donknap/dpanel/app/ctrl/command/store"
	"github.com/donknap/dpanel/app/ctrl/command/user"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/console"
)

type Provider struct {
}

func (provider *Provider) Register(console console.Console) {
	console.RegisterCommand(new(user.Reset))
	console.RegisterCommand(new(store.Sync))

	// 容器相关
	console.RegisterCommand(new(container.Upgrade))
	console.RegisterCommand(new(container.Backup))

	console.RegisterCommand(new(compose.Deploy))
}
