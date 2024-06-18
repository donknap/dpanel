package main

import (
	"embed"
	_ "embed"
	"github.com/donknap/dpanel/app/application"
	"github.com/donknap/dpanel/app/common"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	common2 "github.com/donknap/dpanel/common/middleware"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go-support/src/facade"
	app "github.com/we7coreteam/w7-rangine-go/src"
	"github.com/we7coreteam/w7-rangine-go/src/http"
	"github.com/we7coreteam/w7-rangine-go/src/http/middleware"
	"os"
)

var (
	//go:embed config.yaml
	ConfigFile []byte
	//go:embed asset
	Asset embed.FS
)

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

	// 同步数据库
	db.Migrator().AutoMigrate(
		&entity.Event{},
		&entity.Image{},
		&entity.Notice{},
		&entity.Registry{},
		&entity.Setting{},
		&entity.Site{},
		&entity.SiteDomain{},
		&entity.Compose{},
	)
	// 如果没有管理配置新建一条
	founderSetting, _ := dao.Setting.
		Where(dao.Setting.GroupName.Eq(logic.SettingUser)).
		Where(dao.Setting.Name.Eq(logic.SettingUserFounder)).First()
	if founderSetting == nil {
		dao.Setting.Create(&entity.Setting{
			GroupName: logic.SettingUser,
			Name:      logic.SettingUserFounder,
			Value: &accessor.SettingValueOption{
				"password": "f6fdffe48c908deb0f4c3bd36c032e72",
				"username": "admin",
			},
		})
	}
	// 初始化挂载目录
	for _, path := range []string{
		"nginx/default_host",
		"nginx/proxy_host",
		"nginx/redirection_host",
		"nginx/dead_host",
		"nginx/temp",
		"nginx/cert",
		"storage",
	} {
		os.MkdirAll(facade.GetConfig().GetString("storage.local.path")+"/"+path, os.ModePerm)
	}

	// 注册资源
	facade.GetContainer().NamedSingleton("asset", func() embed.FS {
		return Asset
	})

	// 全局登录判断
	httpServer.Use(common2.AuthMiddleware{}.Process)

	// 注册业务 provider，此模块中需要使用 http server 和 console
	new(common.Provider).Register(httpServer)
	new(application.Provider).Register(httpServer)
	app.RunConsole()
}
