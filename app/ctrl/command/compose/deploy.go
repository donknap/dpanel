package compose

import (
	"fmt"

	"github.com/donknap/dpanel/app/ctrl/sdk/proxy"
	"github.com/donknap/dpanel/app/ctrl/sdk/types/app"
	"github.com/donknap/dpanel/app/ctrl/sdk/utils"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"github.com/we7coreteam/w7-rangine-go/v2/src/console"
)

type Deploy struct {
	console.Abstract
}

func (self Deploy) GetName() string {
	return "compose:deploy"
}

func (self Deploy) GetDescription() string {
	return "Upgrade or rebuild compose task"
}

func (self Deploy) Configure(command *cobra.Command) {
	command.Flags().String("docker-env", "local", "Docker server name")
	command.Flags().String("name", "", "Compose task name")
	command.Flags().String("pull-image", "command", `Methods for pulling images ("dpanel"|"command")`)
	_ = command.MarkFlagRequired("name")
}

func (self Deploy) Handle(cmd *cobra.Command, args []string) {
	name, _ := cmd.Flags().GetString("name")
	dockerEnv, _ := cmd.Flags().GetString("docker-env")
	pullImage, _ := cmd.Flags().GetString("pull-image")

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

	composeTask, err := proxyClient.AppComposeTask(name)
	if err != nil {
		utils.Result{}.Error(err)
		return
	}
	if composeTask.Detail.Setting.Type != accessor.ComposeTypeOutPath {
		err = proxyClient.CommonExplorerSyncDPanel(composeTask.Detail.Name)
		if err != nil {
			utils.Result{}.Error(err)
			return
		}
	}
	if pullImage == "dpanel" {
		for _, item := range composeTask.Project.Services {
			_, err = proxyClient.AppImageTagRemote(&app.ImageTagRemoteOption{
				Tag:  item.Image,
				Type: "pull",
			})
			if err != nil {
				utils.Result{}.Error(err)
				return
			}
		}
	}

	err = proxyClient.AppComposeDeploy(&app.ComposeDeployOption{
		Id:         fmt.Sprintf("%d", composeTask.Detail.ID),
		CreatePath: false,
	})
	if err != nil {
		utils.Result{}.Error(err)
		return
	}
	utils.Result{}.Success(gin.H{
		"name": name,
	})
	return
}
