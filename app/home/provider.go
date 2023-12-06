package home

import (
	"github.com/donknap/dpanel/app/home/command"
	"github.com/donknap/dpanel/app/home/http/controller"
	"github.com/donknap/dpanel/app/home/http/middleware"
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

	// 注册一些路由
	httpServer.RegisterRouters(func(engine *gin.Engine) {
		engine.GET("/home/index", middleware.Home{}.Process, controller.Home{}.Index)
	})

	httpServer.RegisterRouters(func(engine *gin.Engine) {
		cors := engine.Group("/", common.CorsMiddleware{}.Process)
		cors.GET("/home/mock/user-info", middleware.Home{}.Process, controller.Mock{}.UserInfo)
		cors.GET("/home/mock/error", middleware.Home{}.Process, controller.Mock{}.Error)

		cors.POST("/home/attach/upload", middleware.Home{}.Process, controller.Attach{}.Upload)
	})

	httpServer.RegisterRouters(func(engine *gin.Engine) {
		engine.GET("/home/ws", middleware.Home{}.Process, controller.Home{}.Ws)
	})
}
