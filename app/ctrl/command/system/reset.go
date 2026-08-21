package system

import (
	"github.com/donknap/dpanel/app/ctrl/sdk/proxy"
	"github.com/donknap/dpanel/app/ctrl/sdk/types/common"
	"github.com/donknap/dpanel/app/ctrl/sdk/utils"
	"github.com/gookit/color"
	"github.com/spf13/cobra"
)

type Reset struct{}

func (self Reset) GetName() string {
	return "system:reset"
}

func (self Reset) GetDescription() string {
	return "Reset system credentials, security entrance, cache, or online users"
}

func (self Reset) Configure(cmd *cobra.Command) {
	cmd.Flags().String("user", "", "Reset the user; without a value, reset admin with a random password")
	userFlag := cmd.Flags().Lookup("user")
	userFlag.NoOptDefVal = "admin"
	cmd.Flags().String("password", "", "Set the password; omitted with --user generates a random password")
	cmd.Flags().String("entrance", "", "Set security entrance: random, none, or a relative path")
	entranceFlag := cmd.Flags().Lookup("entrance")
	entranceFlag.NoOptDefVal = "random"
	cmd.Flags().Bool("cache", false, "Clear rebuildable caches, notices, Docker events, and temporary files")
	cmd.Flags().Bool("online-user", false, "Invalidate all online users")
}

func (self Reset) Handle(cmd *cobra.Command, args []string) {
	user, _ := cmd.Flags().GetString("user")
	password, _ := cmd.Flags().GetString("password")
	entrance, _ := cmd.Flags().GetString("entrance")
	clearCache, _ := cmd.Flags().GetBool("cache")
	onlineUser, _ := cmd.Flags().GetBool("online-user")

	if !cmd.Flags().Changed("user") && password != "" {
		color.Errorln("--password requires --user")
		return
	}
	if !cmd.Flags().Changed("user") && !cmd.Flags().Changed("entrance") && !clearCache && !onlineUser {
		color.Errorln("at least one reset option is required")
		return
	}
	if cmd.Flags().Changed("user") && user == "" {
		user = "admin"
	}

	proxyClient, err := proxy.NewProxyClient()
	if err != nil {
		color.Errorln("Error:", err)
		return
	}
	result, err := proxyClient.CommonReset(common.ResetOption{
		User:       user,
		Password:   password,
		Entrance:   entrance,
		Cache:      clearCache,
		OnlineUser: onlineUser,
	})
	if err != nil {
		color.Errorln("Error:", err)
		return
	}
	utils.Result{}.Success(result)
}
