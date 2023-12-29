package main

import (
	"embed"
	_ "embed"
	"github.com/donknap/dpanel/app/application"
	"github.com/donknap/dpanel/app/common"
	"github.com/donknap/dpanel/common/dao"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/we7coreteam/w7-rangine-go-support/src/facade"
	app "github.com/we7coreteam/w7-rangine-go/src"
	"github.com/we7coreteam/w7-rangine-go/src/http"
	"github.com/we7coreteam/w7-rangine-go/src/http/middleware"
	"path/filepath"
)

var (
	//go:embed config.yaml
	ConfigFile []byte
	//go:embed asset
	Asset       embed.FS
	RootPath, _ = filepath.Abs("./")
)

type provider struct {
	ConfigDefault func(viper *viper.Viper)
	EventDault    func()
}

func main() {
	app := app.NewApp(
		app.Option{
			Name: "w7-rangine-go-skeleton",
		},
	)
	// 业务中需要使用 http server，这里需要先实例化
	httpServer := new(http.Provider).Register(app.GetConfig(), app.GetConsole(), app.GetServerManager()).Export()
	// 注册一些全局中间件，路由或是其它一些全局操作
	httpServer.Use(middleware.GetPanicHandlerMiddleware())
	httpServer.RegisterRouters(
		func(engine *gin.Engine) {
			engine.NoRoute(
				func(context *gin.Context) {
					context.String(404, "404 Not Found")
				},
			)
		},
	)

	db, err := facade.GetDbFactory().Channel("default")
	if err != nil {
		panic(err)
	}
	dao.SetDefault(db)

	// 注册资源
	facade.GetContainer().NamedSingleton("asset", func() embed.FS {
		return Asset
	})

	// 注册业务 provider，此模块中需要使用 http server 和 console
	new(common.Provider).Register(httpServer, app.GetConsole())
	new(application.Provider).Register(httpServer, app.GetConsole())
	app.RunConsole()
}
