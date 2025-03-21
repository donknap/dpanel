package container

import (
	"encoding/json"
	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/app/ctrl/logic"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
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
	return "检测当前容器更新"
}

func (self Upgrade) Configure(command *cobra.Command) {
	command.Flags().String("name", "", "容器名称")
	command.Flags().String("docker-env", "", "指定 docker 环境")
	command.Flags().Bool("upgrade", false, "是否升级容器")
	_ = command.MarkFlagRequired("name")
}

func (self Upgrade) Handle(cmd *cobra.Command, args []string) {
	name, _ := cmd.Flags().GetString("name")
	dockerEnv, _ := cmd.Flags().GetString("docker-env")

	code, err := logic.User{}.GetAuth(time.Now().Add(time.Minute))
	if err != nil {
		logic.Result{}.Error(err)
		return
	}
	if dockerEnv == "" {
		dockerEnv = docker.DefaultClientName
	}
	out, _, err := logic.Proxy{}.Post("/api/common/env/switch", code, gin.H{
		"name": dockerEnv,
	})
	if err != nil {
		logic.Result{}.Error(err)
		return
	}
	_, raw, err := logic.Proxy{}.Post("/api/app/container/get-detail", code, gin.H{
		"md5": name,
	})
	if err != nil {
		logic.Result{}.Error(err)
		return
	}
	data := struct {
		Data struct {
			Info container.InspectResponse `json:"info"`
		} `json:"data"`
	}{}
	err = json.Unmarshal(raw, &data)
	if err != nil {
		logic.Result{}.Error(err)
		return
	}
	out, _, err = logic.Proxy{}.Post("/api/app/image/check-upgrade", code, gin.H{
		"tag": data.Data.Info.Config.Image,
		"md5": data.Data.Info.Image,
	})
	if err != nil {
		logic.Result{}.Error(err)
		return
	}
	logic.Result{}.Success(out.Data)
	return
}
