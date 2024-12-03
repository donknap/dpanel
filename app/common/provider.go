package common

import (
	"github.com/docker/docker/client"
	"github.com/donknap/dpanel/app/common/http/controller"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	common "github.com/donknap/dpanel/common/middleware"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
	http_server "github.com/we7coreteam/w7-rangine-go/v2/src/http/server"
	"net/http"
)

type Provider struct {
}

func (provider *Provider) Register(httpServer *http_server.Server) {

	httpServer.RegisterRouters(func(engine *gin.Engine) {
		cors := engine.Group("/api", common.CorsMiddleware{}.Process)

		cors.POST("/common/attach/upload", controller.Attach{}.Upload)
		cors.POST("/common/attach/delete", controller.Attach{}.Delete)

		// 仓库相关
		cors.POST("/common/registry/create", controller.Registry{}.Create)
		cors.POST("/common/registry/get-list", controller.Registry{}.GetList)
		cors.POST("/common/registry/get-detail", controller.Registry{}.GetDetail)
		cors.POST("/common/registry/update", controller.Registry{}.Update)
		cors.POST("/common/registry/delete", controller.Registry{}.Delete)

		// 全局
		cors.POST("/common/event/get-list", controller.Event{}.GetList)
		cors.POST("/common/event/prune", controller.Event{}.Prune)

		cors.POST("/common/notice/unread", controller.Notice{}.Unread)
		cors.POST("/common/notice/get-list", controller.Notice{}.GetList)
		cors.POST("/common/notice/delete", controller.Notice{}.Delete)

		// 用户
		cors.POST("/common/user/login", controller.User{}.Login)
		cors.POST("/common/user/login-info", controller.User{}.LoginInfo)
		cors.POST("/common/user/get-user-info", controller.User{}.GetUserInfo)

		// 配置
		cors.POST("/common/setting/save", controller.Setting{}.Save)
		cors.POST("/common/setting/founder", controller.Setting{}.Founder)
		cors.POST("/common/setting/get-setting", controller.Setting{}.GetSetting)

		cors.POST("/common/home/info", controller.Home{}.Info)
		cors.POST("/common/home/usage", controller.Home{}.Usage)
		cors.POST("/common/home/upgrade-script", controller.Home{}.UpgradeScript)
		cors.POST("/common/home/get-stat-list", controller.Home{}.GetStatList)

		// 环境管理
		cors.POST("/common/env/get-list", controller.Env{}.GetList)
		cors.POST("/common/env/create", controller.Env{}.Create)
		cors.POST("/common/env/switch", controller.Env{}.Switch)
		cors.POST("/common/env/delete", controller.Env{}.Delete)

		// 应用商店
		cors.POST("/common/store/create", controller.Store{}.Create)
		cors.POST("/common/store/get-list", controller.Store{}.GetList)
		cors.POST("/common/store/delete", controller.Store{}.Delete)
		cors.POST("/common/store/sync", controller.Store{}.Sync)
		cors.POST("/common/store/deploy", controller.Store{}.Deploy)

		engine.StaticFS("/dpanel/static/store/file", http.FS(logic.StoreLogoFileSystem{}))
	})

	httpServer.RegisterRouters(func(engine *gin.Engine) {
		wsCors := engine.Group("/ws/", common.CorsMiddleware{}.Process)

		wsCors.GET("/common/notice", controller.Home{}.WsNotice)
		wsCors.GET("/common/console/:id", controller.Home{}.WsConsole)
	})

	// 当前如果有连接，则添加一条docker环境数据
	_, err := logic.DockerEnv{}.GetEnvByName("local")
	if err != nil {
		logic.DockerEnv{}.UpdateEnv(&accessor.DockerClientResult{
			Name:    "local",
			Title:   "本机",
			Address: client.DefaultDockerHost,
			Default: true,
		})
	}
	_, err = docker.Sdk.Client.Info(docker.Sdk.Ctx)
	if err == nil {
		go logic.EventLogic{}.MonitorLoop()
	}
}
