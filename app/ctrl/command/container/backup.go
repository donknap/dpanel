package container

import (
	"github.com/donknap/dpanel/app/ctrl/sdk/proxy"
	"github.com/donknap/dpanel/app/ctrl/sdk/types/app"
	"github.com/donknap/dpanel/app/ctrl/sdk/utils"
	"github.com/spf13/cobra"
	"github.com/we7coreteam/w7-rangine-go/v2/src/console"
)

type Backup struct {
	console.Abstract
}

func (self Backup) GetName() string {
	return "container:backup"
}

func (self Backup) GetDescription() string {
	return "生成容器快照"
}

func (self Backup) Configure(command *cobra.Command) {
	command.Flags().String("name", "", "容器名称")
	command.Flags().Int("enable-image", 0, "是否快照容器镜像")
	command.Flags().String("docker-env", "", "指定 docker 环境")
	_ = command.MarkFlagRequired("name")
}

func (self Backup) Handle(cmd *cobra.Command, args []string) {
	name, _ := cmd.Flags().GetString("name")
	dockerEnv, _ := cmd.Flags().GetString("docker-env")
	enableImage, _ := cmd.Flags().GetInt("enable-image")

	proxyClient, err := proxy.NewProxyClient()
	if err != nil {
		utils.Result{}.Error(err)
		return
	}
	dockerEnvList, err := proxyClient.CommonEnvGetList()
	if err != nil {
		utils.Result{}.Error(err)
		return
	}
	if dockerEnv != "" && dockerEnv != dockerEnvList.CurrentName {
		err = proxyClient.CommonEnvSwitch(dockerEnv)
		if err != nil {
			utils.Result{}.Error(err)
			return
		}
		defer func() {
			_ = proxyClient.CommonEnvSwitch(dockerEnvList.CurrentName)
		}()
	}
	result, err := proxyClient.AppContainerBackupCreate(&app.ContainerBackupOption{
		Id:                name,
		EnableImage:       enableImage > 0,
		EnableCommitImage: false,
		Volume:            make([]string, 0),
	})
	if err != nil {
		utils.Result{}.Error(err)
		return
	}
	utils.Result{}.Success(result)
	return
}
