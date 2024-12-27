package container

import (
	"encoding/json"
	"github.com/docker/docker/api/types"
	"github.com/donknap/dpanel/app/ctrl/logic"
	"github.com/gin-gonic/gin"
	"github.com/gookit/color"
	"github.com/spf13/cobra"
	"github.com/we7coreteam/w7-rangine-go/v2/src/console"
	"time"
)

type Upgrade struct {
	console.Abstract
}

func (self Upgrade) GetName() string {
	return "container:upgrade"
}

func (self Upgrade) GetDescription() string {
	return "拉取最新的镜像升级当前容器"
}

func (self Upgrade) Configure(command *cobra.Command) {
	command.Flags().String("name", "", "容器名称")
	command.Flags().String("docker-env", "", "指定 docker 环境")
	_ = command.MarkFlagRequired("name")
}

func (self Upgrade) Handle(cmd *cobra.Command, args []string) {
	name, _ := cmd.Flags().GetString("name")
	dockerEnv, _ := cmd.Flags().GetString("docker-env")

	code, err := logic.User{}.GetAuth(time.Now().Add(time.Minute))
	if err != nil {
		color.Error.Println(err)
		return
	}
	out, _, err := logic.Proxy{}.Post("/api/common/env/switch", code, gin.H{
		"name": dockerEnv,
	})
	if err != nil {
		color.Errorln(err)
		return
	}
	_, raw, err := logic.Proxy{}.Post("/api/app/container/get-detail", code, gin.H{
		"md5": name,
	})
	if err != nil {
		color.Errorln(err)
		return
	}
	data := struct {
		Data struct {
			Info types.ContainerJSON `json:"info"`
		} `json:"data"`
	}{}
	err = json.Unmarshal(raw, &data)
	if err != nil {
		color.Errorln(err)
		return
	}
	out, _, err = logic.Proxy{}.Post("/api/app/image/check-upgrade", code, gin.H{
		"tag": data.Data.Info.Config.Image,
		"md5": data.Data.Info.Image,
	})
	if err != nil {
		color.Error.Println(err)
		return
	}
	hasUpgrade := out.Data.(map[string]interface{})["upgrade"].(bool)
	if !hasUpgrade {
		color.Error.Println("当前容器无可用更新镜像")
		return
	}
}
