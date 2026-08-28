package system

import (
	"errors"
	"strings"

	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/app/ctrl/sdk/proxy"
	"github.com/donknap/dpanel/app/ctrl/sdk/types/common"
	"github.com/donknap/dpanel/app/ctrl/sdk/utils"
	"github.com/donknap/dpanel/common/dao"
	"github.com/google/uuid"
	"github.com/gookit/color"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

type Reset struct{}

func (self Reset) GetName() string {
	return "system:reset"
}

func (self Reset) GetDescription() string {
	return "Reset system credentials, security entrance, cache, or online users"
}

func (self Reset) Configure(cmd *cobra.Command) {
	cmd.Flags().String("username", "", "Reset the username; without a value, reset admin with a random password")
	usernameFlag := cmd.Flags().Lookup("username")
	usernameFlag.NoOptDefVal = "admin"
	cmd.Flags().String("password", "", "Set the password; omitted with --username generates a random password")
	cmd.Flags().String("entrance", "", "Set security entrance: no value for random, none to disable, or a relative path")
	entranceFlag := cmd.Flags().Lookup("entrance")
	entranceFlag.NoOptDefVal = uuid.New().String()[24:30]
	cmd.Flags().Bool("cache", false, "Clear rebuildable caches, notices, Docker events, and temporary files")
	cmd.Flags().Bool("online-user", false, "Invalidate all online users")
}

// ResetFounderUser updates the founder account directly in the local settings.
// It intentionally avoids the HTTP API so it can initialize an instance before
// an authenticated founder account exists.
func ResetFounderUser(username string, password string) (string, string, error) {
	founder, err := dao.Setting.
		Where(dao.Setting.GroupName.Eq(logic.SettingGroupUser)).
		Where(dao.Setting.Name.Eq(logic.SettingGroupUserFounder)).First()
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", "", err
	}

	if username == "" {
		if founder != nil && founder.Value != nil {
			username = founder.Value.Username
		} else {
			username = "admin"
		}
	}
	if password == "" {
		password = uuid.New().String()[24:]
	}

	if founder == nil {
		if _, err := (logic.User{}).CreateFounderUser(username, password); err != nil {
			return "", "", err
		}
		return username, password, nil
	}
	if founder.Value == nil {
		return "", "", errors.New("founder user data is invalid")
	}

	founder.Value.Username = username
	founder.Value.Password = (logic.User{}).GetMd5Password(password, username)
	if err := dao.Setting.Save(founder); err != nil {
		return "", "", err
	}
	return username, password, nil
}

func (self Reset) Handle(cmd *cobra.Command, args []string) {
	username, _ := cmd.Flags().GetString("username")
	password, _ := cmd.Flags().GetString("password")
	entranceValue, _ := cmd.Flags().GetString("entrance")
	clearCache, _ := cmd.Flags().GetBool("cache")
	onlineUser, _ := cmd.Flags().GetBool("online-user")
	resetUser := cmd.Flags().Changed("username")

	if !resetUser && password != "" {
		color.Errorln("--password requires --username")
		return
	}
	if !resetUser && !cmd.Flags().Changed("entrance") && !clearCache && !onlineUser {
		color.Errorln("at least one reset option is required")
		return
	}

	var resetUsername, resetPassword string
	if resetUser {
		if username == "" {
			username = "admin"
		}
		var err error
		resetUsername, resetPassword, err = ResetFounderUser(username, password)
		if err != nil {
			color.Errorln("Error:", err)
			return
		}
	}

	resetOther := cmd.Flags().Changed("entrance") || clearCache || onlineUser
	if !resetOther {
		utils.Result{}.Success(map[string]any{
			"username": resetUsername,
			"password": resetPassword,
		})
		return
	}

	var entrance *string
	if cmd.Flags().Changed("entrance") {
		if strings.EqualFold(entranceValue, "none") {
			entranceValue = ""
		}
		entrance = &entranceValue
	}

	proxyClient, err := proxy.NewProxyClient()
	if err != nil {
		color.Errorln("Error:", err)
		return
	}
	result, err := proxyClient.CommonReset(common.ResetOption{
		Entrance:   entrance,
		Cache:      clearCache,
		OnlineUser: onlineUser,
	})
	if err != nil {
		color.Errorln("Error:", err)
		return
	}
	if resetUser {
		result["username"] = resetUsername
		result["password"] = resetPassword
	}
	utils.Result{}.Success(result)
}
