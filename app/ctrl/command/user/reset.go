package user

import (
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
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
	return "重置面板用户名或是密码"
}

func (self Reset) Configure(command *cobra.Command) {
	command.Flags().String("password", "", "重置管理员密码")
	command.Flags().String("username", "", "重置管理员用户名")
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
		color.Errorln("重置失败，", err.Error())
		return
	}
	password, err := cmd.Flags().GetString("password")
	if err != nil {
		color.Errorln("重置失败，", err.Error())
		return
	}
	if username == "" && password == "" {
		username = "admin"
		password = function.GetRandomString(10)
	}

	if username != "" && password == "" {
		color.Errorln("重置用户名时必须指定密码")
		return
	}

	if username != "" {
		founder.Value.Username = username
	}

	founder.Value.Password = logic.User{}.GetMd5Password(password, founder.Value.Username)

	err = dao.Setting.Save(founder)
	if err != nil {
		color.Errorln("重置失败，", err.Error())
		return
	}
	color.Println("用户名: ", username)
	color.Println("密  码: ", password)
	color.Successln("用户名或是密码重置成功")
}
