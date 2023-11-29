package application

import (
	"github.com/donknap/dpanel/app/application/command"
	"github.com/donknap/dpanel/app/application/http/controller"
	"github.com/donknap/dpanel/app/application/logic"
	common "github.com/donknap/dpanel/common/middleware"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go-support/src/console"
	"github.com/we7coreteam/w7-rangine-go-support/src/facade"
	http_server "github.com/we7coreteam/w7-rangine-go/src/http/server"
)

type Provider struct {
}

func (provider *Provider) Register(httpServer *http_server.Server, console console.Console) {
	provider.initContainerTask()

	// 注册一个 application:test 命令
	console.RegisterCommand(new(command.Test))

	task := logic.NewContainerTask()
	go task.CreateLoop()

	// 注册一些路由
	httpServer.RegisterRouters(
		func(engine *gin.Engine) {
			cors := engine.Group("/", common.CorsMiddleware{}.Process)

			cors.POST("/app/run-env/source-env", controller.RunEnv{}.SupportRunEnv)
			cors.POST("/app/run-env/php-ext", controller.RunEnv{}.PhpExt)

			cors.POST("/app/site/create-by-image", controller.Site{}.CreateByImage)
			cors.POST("/app/site/get-list", controller.Site{}.GetList)
		},
	)
}

func (provider Provider) initContainerTask() {
	err := facade.GetContainer().NamedSingleton("containerTask", func() *logic.ContainerTask {
		obj := &logic.ContainerTask{}
		obj.QueueCreate = make(chan *logic.CreateMessage)
		return obj
	})
	if err != nil {
		panic(err)
	}
}
