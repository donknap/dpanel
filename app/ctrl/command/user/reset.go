package user

import (
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/google/uuid"
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
	return "Reset the Admin username or password."
}

func (self Reset) Configure(command *cobra.Command) {
	command.Flags().String("password", "", "Reset password")
	command.Flags().String("username", "", "Reset username")
}

func (self Reset) Handle(cmd *cobra.Command, args []string) {
	founder, _ := dao.Setting.
		Where(dao.Setting.GroupName.Eq(logic.SettingGroupUser)).
		Where(dao.Setting.Name.Eq(logic.SettingGroupUserFounder)).First()
	if founder == nil {
		founder = &entity.Setting{
			GroupName: logic.SettingGroupUser,
			Name:      logic.SettingGroupUserFounder,
			Value: &accessor.SettingValueOption{
				Username: "",
				Password: "",
			},
		}
	}
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
	if username == "" && password == "" {
		username = "admin"
		password = uuid.New().String()[24:]
	}

	if username != "" && password == "" {
		color.Errorln("When resetting the username, the password must also be reset.")
		return
	}

	if username != "" {
		founder.Value.Username = username
	}

	founder.Value.Password = logic.User{}.GetMd5Password(password, founder.Value.Username)

	err = dao.Setting.Save(founder)
	if err != nil {
		color.Errorln("Error: ", err.Error())
		return
	}
	color.Println("用户名 (Username): ", username)
	color.Println("密  码 (Password): ", password)
	color.Successln("Success")
}
