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
	return "升级重建 compose 任务"
}

func (self Deploy) Configure(command *cobra.Command) {
	command.Flags().String("docker-env", "", "指定 docker 环境")
	command.Flags().String("name", "", "compose 任务名称")
	command.Flags().StringArray("environment", make([]string, 0), "配置 compose 任务环境变量")
	command.Flags().StringArray("service-name", make([]string, 0), "指定创建的 service 名称")
	command.Flags().Int("remove-orphans", 0, "清理已重命名或是删除的服务容器")
	command.Flags().String("pull-image", "", "拉取镜像的方式 dpanel command")
	_ = command.MarkFlagRequired("name")
}

func (self Deploy) Handle(cmd *cobra.Command, args []string) {
	name, _ := cmd.Flags().GetString("name")
	dockerEnv, _ := cmd.Flags().GetString("docker-env")
	removeOrphans, _ := cmd.Flags().GetInt("remove-orphans")
	environment, _ := cmd.Flags().GetStringArray("environment")
	serviceName, _ := cmd.Flags().GetStringArray("service-name")
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
		DeployServiceName: serviceName,
		CreatePath:        false,
		RemoveOrphans:     removeOrphans > 0,
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
