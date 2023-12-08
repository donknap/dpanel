package common

import (
	"github.com/donknap/dpanel/app/common/command"
	"github.com/donknap/dpanel/app/common/http/controller"
	"github.com/donknap/dpanel/app/common/http/middleware"
	common "github.com/donknap/dpanel/common/middleware"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go-support/src/console"
	http_server "github.com/we7coreteam/w7-rangine-go/src/http/server"
)

type Provider struct {
}

func (provider *Provider) Register(httpServer *http_server.Server, console console.Console) {
	// 注册一个 home:test 命令
	console.RegisterCommand(new(command.Test))

	httpServer.RegisterRouters(func(engine *gin.Engine) {
		cors := engine.Group("/", common.CorsMiddleware{}.Process)

		cors.POST("/home/attach/upload", middleware.Home{}.Process, controller.Attach{}.Upload)

		// 仓库相关
		cors.POST("/common/registry/create", middleware.Home{}.Process, controller.Registry{}.Create)
	})

	httpServer.RegisterRouters(func(engine *gin.Engine) {
		engine.GET("/home/index", middleware.Home{}.Process, controller.Home{}.Index)
		engine.GET("/home/ws", middleware.Home{}.Process, controller.Home{}.Ws)
	})
}
