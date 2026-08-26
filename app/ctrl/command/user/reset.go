package user

import (
	"github.com/donknap/dpanel/app/ctrl/sdk/proxy"
	"github.com/donknap/dpanel/app/ctrl/sdk/types/common"
	"github.com/donknap/dpanel/app/ctrl/sdk/utils"
	"github.com/gookit/color"
	"github.com/spf13/cobra"
	"github.com/we7coreteam/w7-rangine-go/v2/src/console"
)

type Reset struct {
	console.Abstract
}

func (self Reset) GetName() string {
	return "user:reset"
}

func (self Reset) GetDescription() string {
	return "Reset the Admin username or password (compatibility command)."
}

func (self Reset) Configure(command *cobra.Command) {
	command.Deprecated = "use system:reset --username instead"
	command.Flags().String("password", "", "Reset password")
	command.Flags().String("username", "", "Reset username")
}

func (self Reset) Handle(cmd *cobra.Command, args []string) {
	username, err := cmd.Flags().GetString("username")
	if err != nil {
		color.Errorln("Error: ", err.Error())
		return
	}
	password, err := cmd.Flags().GetString("password")
	if err != nil {
		color.Errorln("Error: ", err.Error())
		return
	}
	if username != "" && password == "" {
		color.Errorln("When resetting the username, the password must also be reset.")
		return
	}
	if username == "" && password == "" {
		username = "admin"
	}

	client, err := proxy.NewProxyClient()
	if err != nil {
		color.Errorln("Error: ", err.Error())
		return
	}
	result, err := client.CommonReset(common.ResetOption{
		User:     username,
		Password: password,
	})
	if err != nil {
		color.Errorln("Error: ", err.Error())
		return
	}
	utils.Result{}.Success(result)
}
