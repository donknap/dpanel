package common

import (
	"github.com/donknap/dpanel/app/common/http/controller"
	common "github.com/donknap/dpanel/common/middleware"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go-support/src/console"
	http_server "github.com/we7coreteam/w7-rangine-go/src/http/server"
)

type Provider struct {
}

func (provider *Provider) Register(httpServer *http_server.Server, console console.Console) {

	httpServer.RegisterRouters(func(engine *gin.Engine) {
		cors := engine.Group("/", common.CorsMiddleware{}.Process)

		cors.POST("/common/attach/upload", controller.Attach{}.Upload)
		cors.POST("/common/attach/delete", controller.Attach{}.Delete)

		// 仓库相关
		cors.POST("/common/registry/create", controller.Registry{}.Create)
		cors.POST("/common/registry/get-list", controller.Registry{}.GetList)

		// 全局
		cors.POST("/common/event/get-list", controller.Event{}.GetList)

		cors.POST("/common/notice/unread", controller.Notice{}.Unread)
		cors.POST("/common/notice/get-list", controller.Notice{}.GetList)
		cors.POST("/common/notice/delete", controller.Notice{}.Delete)
	})

	httpServer.RegisterRouters(func(engine *gin.Engine) {
		engine.GET("/home/index", controller.Home{}.Index)
		engine.GET("/home/ws", controller.Home{}.Ws)
	})

	//event := logic.EventLogic{}
	//go event.MonitorLoop()
}
