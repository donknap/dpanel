package container

import (
	"github.com/donknap/dpanel/app/ctrl/sdk/proxy"
	"github.com/donknap/dpanel/app/ctrl/sdk/types/app"
	"github.com/donknap/dpanel/app/ctrl/sdk/utils"
	"github.com/spf13/cobra"
	"github.com/we7coreteam/w7-rangine-go/v2/src/console"
)

type Upgrade struct {
	console.Abstract
}

func (self Upgrade) GetName() string {
	return "container:upgrade"
}

func (self Upgrade) GetDescription() string {
	return "Checking for container updates"
}

func (self Upgrade) Configure(command *cobra.Command) {
	command.Flags().String("name", "", "Container name")
	command.Flags().String("docker-env", "local", "Specify the Docker Server")
	command.Flags().Int("upgrade", 0, `Want to upgrade the container? ("1"|"0")`)
	_ = command.MarkFlagRequired("name")
}

func (self Upgrade) Handle(cmd *cobra.Command, args []string) {
	name, _ := cmd.Flags().GetString("name")
	dockerEnv, _ := cmd.Flags().GetString("docker-env")
	isUpgrade, _ := cmd.Flags().GetInt("upgrade")

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
	containerInfo, err := proxyClient.AppContainerGetDetail(name)
	if err != nil {
		utils.Result{}.Error(err)
		return
	}
	result, err := proxyClient.AppImageCheckUpgrade(&app.ImageCheckUpgradeOption{
		Tag:       containerInfo.Info.Config.Image,
		Md5:       containerInfo.Info.Image,
		CacheTime: 0,
	})
	if err != nil {
		utils.Result{}.Error(err)
		return
	}
	if isUpgrade <= 0 {
		utils.Result{}.Success(result)
		return
	}
	_, err = proxyClient.AppImageTagRemote(&app.ImageTagRemoteOption{
		Tag:  containerInfo.Info.Config.Image,
		Type: "pull",
	})
	if err != nil {
		utils.Result{}.Error(err)
		return
	}
	containerUpgradeResult, err := proxyClient.AppContainerUpgrade(&app.ContainerUpgradeOption{
		Md5:       containerInfo.Info.ID,
		EnableBak: true,
	})

	if err != nil {
		utils.Result{}.Error(err)
		return
	}

	utils.Result{}.Success(containerUpgradeResult)
	return
}
