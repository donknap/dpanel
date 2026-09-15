package ctrl

import (
	"github.com/donknap/dpanel/app/ctrl/command/compose"
	"github.com/donknap/dpanel/app/ctrl/command/container"
	"github.com/donknap/dpanel/app/ctrl/command/store"
	"github.com/donknap/dpanel/app/ctrl/command/system"
	"github.com/donknap/dpanel/app/ctrl/command/user"
	"github.com/donknap/dpanel/app/ctrl/sdk/utils"
	"github.com/spf13/cobra"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/console"
)

type Provider struct {
}

func (provider Provider) Command(console console.Console) {
	if rootConsole, ok := console.(interface{ GetHandler() *cobra.Command }); ok {
		rootCommand := rootConsole.GetHandler()
		rootCommand.PersistentFlags().BoolP("quiet", "q", false, "Suppress command result output")
		previousPersistentPreRun := rootCommand.PersistentPreRun
		rootCommand.PersistentPreRun = func(cmd *cobra.Command, args []string) {
			if previousPersistentPreRun != nil {
				previousPersistentPreRun(cmd, args)
			}
			quiet, _ := cmd.Flags().GetBool("quiet")
			utils.SetQuiet(quiet)
		}
	}

	console.RegisterCommand(new(user.Reset))
	console.RegisterCommand(new(store.Sync))

	console.RegisterCommand(new(container.Upgrade))
	console.RegisterCommand(new(container.Backup))

	console.RegisterCommand(new(compose.Deploy))

	console.RegisterCommand(new(system.Backup))
	console.RegisterCommand(new(system.Cache))
	console.RegisterCommand(new(system.Info))
	console.RegisterCommand(new(system.Notice))
	console.RegisterCommand(new(system.Prune))
	console.RegisterCommand(new(system.Reset))
}
