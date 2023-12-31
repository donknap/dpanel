package application

import (
	"github.com/donknap/dpanel/app/application/command"
	"github.com/donknap/dpanel/app/application/http/controller"
	common "github.com/donknap/dpanel/common/middleware"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go-support/src/console"
	http_server "github.com/we7coreteam/w7-rangine-go/src/http/server"
)

type Provider struct {
}

func (provider *Provider) Register(httpServer *http_server.Server, console console.Console) {
	// 注册一个 application:test 命令
	console.RegisterCommand(new(command.Test))

	// 注册一些路由
	httpServer.RegisterRouters(
		func(engine *gin.Engine) {
			cors := engine.Group("/", common.CorsMiddleware{}.Process)

			cors.POST("/app/run-env/source-env", controller.RunEnv{}.SupportRunEnv)
			cors.POST("/app/run-env/php-ext", controller.RunEnv{}.PhpExt)

			// 站点相关
			cors.POST("/app/site/create-by-image", controller.Site{}.CreateByImage)
			cors.POST("/app/site/get-list", controller.Site{}.GetList)
			cors.POST("/app/site/get-detail", controller.Site{}.GetDetail)
			cors.POST("/app/site/delete", controller.Site{}.Delete)
			cors.POST("/app/site/redeploy", controller.Site{}.ReDeploy)
			cors.POST("/app/site/search-image", controller.Site{}.SearchImage)

			// 容器相关
			cors.POST("/app/container/status", controller.Container{}.Status)
			cors.POST("/app/container/get-list", controller.Container{}.GetList)
			cors.POST("/app/container/get-detail", controller.Container{}.GetDetail)

			// 镜像相关
			cors.POST("/app/image/create-by-dockerfile", controller.Image{}.CreateByDockerfile)
			cors.POST("/app/image/get-list", controller.Image{}.GetList)
			cors.POST("/app/image/get-list-build", controller.Image{}.GetListBuild)
			cors.POST("/app/image/get-detail", controller.Image{}.GetDetail)
			cors.POST("/app/image/get-image-task", controller.Image{}.GetImageTask)
			cors.POST("/app/image/remote", controller.Image{}.Remote)
			cors.POST("/app/image/tag-delete", controller.Image{}.TagDelete)
			cors.POST("/app/image/tag-add", controller.Image{}.TagAdd)
			cors.POST("/app/image/image-delete", controller.Image{}.ImageDelete)
			cors.POST("/app/image/image-prune", controller.Image{}.ImagePrune)
			cors.POST("/app/image/export", controller.Image{}.Export)

			// 文件相关
			engine.POST("/app/explorer/export", controller.Explorer{}.Export)
			engine.POST("/app/explorer/import", controller.Explorer{}.Import)
			engine.POST("/app/explorer/import-file-content", controller.Explorer{}.ImportFileContent)
			engine.POST("/app/explorer/unzip", controller.Explorer{}.Unzip)
			engine.POST("/app/explorer/get-path-list", controller.Explorer{}.GetPathList)
			engine.POST("/app/explorer/delete", controller.Explorer{}.Delete)
			engine.POST("/app/explorer/get-content", controller.Explorer{}.GetContent)

			// 日志相关
			cors.POST("/app/log/run", controller.RunLog{}.Run)

			// 网络相关
			cors.POST("/app/network/get-detail", controller.Network{}.GetDetail)
		},
	)
}
