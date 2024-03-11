package common

import (
	"github.com/donknap/dpanel/app/common/http/controller"
	"github.com/donknap/dpanel/app/common/logic"
	common "github.com/donknap/dpanel/common/middleware"
	"github.com/gin-gonic/gin"
	http_server "github.com/we7coreteam/w7-rangine-go/src/http/server"
)

type Provider struct {
}

func (provider *Provider) Register(httpServer *http_server.Server) {

	httpServer.RegisterRouters(func(engine *gin.Engine) {
		cors := engine.Group("/", common.CorsMiddleware{}.Process)

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
		cors.POST("/common/user/get-user-info", controller.User{}.GetUserInfo)

		// 配置
		cors.POST("/common/setting/save", controller.Setting{}.Save)
		cors.POST("/common/setting/founder", controller.Setting{}.Founder)
		cors.POST("/common/setting/get-setting", controller.Setting{}.GetSetting)
	})

	httpServer.RegisterRouters(func(engine *gin.Engine) {
		engine.GET("/home/index", controller.Home{}.Index)
		engine.GET("/home/ws/notice", controller.Home{}.WsNotice)
		engine.GET("/home/ws/console/:id", controller.Home{}.WsConsole)

		engine.POST("/common/home/info", controller.Home{}.Info)
	})

	event := logic.EventLogic{}
	go event.MonitorLoop()
}
