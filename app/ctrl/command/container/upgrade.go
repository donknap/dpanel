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
	command.Flags().String("name", "", "Name of the container")
	command.Flags().String("docker-env", "local", "Docker server name")
	command.Flags().Bool("upgrade", false, "Upgrade the container to the latest version")
	command.Flags().Bool("enable-bak", true, "Enable backup for the old container")
	command.Flags().Bool("disable-bak", false, "Disable backup for the old container")
	command.Flags().String("image-tag", "", "New image tag to apply")
	_ = command.MarkFlagRequired("name")
}

func (self Upgrade) Handle(cmd *cobra.Command, args []string) {
	name, _ := cmd.Flags().GetString("name")
	dockerEnv, _ := cmd.Flags().GetString("docker-env")
	isUpgrade, _ := cmd.Flags().GetBool("upgrade")
	enableBak, _ := cmd.Flags().GetBool("enable-bak")
	if v, err := cmd.Flags().GetBool("disable-bak"); err == nil && v {
		enableBak = false
	}
	imageName, _ := cmd.Flags().GetString("image-tag")

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
	if !isUpgrade {
		utils.Result{}.Success(result)
		return
	}
	if imageName == "" {
		imageName = containerInfo.Info.Config.Image
	}
	_, err = proxyClient.AppImageTagRemote(&app.ImageTagRemoteOption{
		Tag:  imageName,
		Type: "pull",
	})
	if err != nil {
		utils.Result{}.Error(err)
		return
	}
	containerUpgradeResult, err := proxyClient.AppContainerUpgrade(&app.ContainerUpgradeOption{
		Md5:       containerInfo.Info.ID,
		EnableBak: enableBak,
		ImageTag:  imageName,
	})

	if err != nil {
		utils.Result{}.Error(err)
		return
	}

	utils.Result{}.Success(containerUpgradeResult)
	return
}
