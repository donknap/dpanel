package compose

import (
	"fmt"
	"strings"

	"github.com/donknap/dpanel/app/ctrl/sdk/proxy"
	"github.com/donknap/dpanel/app/ctrl/sdk/types/app"
	"github.com/donknap/dpanel/app/ctrl/sdk/utils"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker/types"
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
	command.Flags().String("docker-env", "", "Specify the Docker Server, default: local")
	command.Flags().String("name", "", "Compose task name")
	command.Flags().StringArrayP("environment", "", make([]string, 0), "Compose task environment, eg: TEST=1")
	command.Flags().String("pull-image", "command", `Methods for pulling images ("dpanel"|"command")`)
	_ = command.MarkFlagRequired("name")
}

func (self Deploy) Handle(cmd *cobra.Command, args []string) {
	name, _ := cmd.Flags().GetString("name")
	dockerEnv, _ := cmd.Flags().GetString("docker-env")
	environment, _ := cmd.Flags().GetStringArray("env")
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
		Id: fmt.Sprintf("%d", composeTask.Detail.ID),
		Environment: function.PluckArrayWalk(environment, func(item string) (types.EnvItem, bool) {
			if k, v, ok := strings.Cut(item, "="); ok {
				return types.EnvItem{
					Name:  k,
					Value: v,
				}, true
			} else {
				return types.EnvItem{}, false
			}
		}),
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
