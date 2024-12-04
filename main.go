package main

import (
	"embed"
	_ "embed"
	"github.com/donknap/dpanel/app/application"
	"github.com/donknap/dpanel/app/common"
	"github.com/donknap/dpanel/app/ctrl"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	common2 "github.com/donknap/dpanel/common/middleware"
	"github.com/donknap/dpanel/common/migrate"
	"github.com/donknap/dpanel/common/service/family"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/gin-gonic/gin"
	"github.com/mcuadros/go-version"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	app "github.com/we7coreteam/w7-rangine-go/v2/src"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/middleware"
	"io/fs"
	"log/slog"
	http2 "net/http"
	"os"
	"path/filepath"
	"runtime"
)

var (
	//go:embed config.yaml
	ConfigFile []byte
	//go:embed asset
	Asset embed.FS
)

func main() {
	// 兼容没有配置存储目录的情况
	if os.Getenv("STORAGE_LOCAL_PATH") == "" {
		exePath, _ := os.Executable()
		_ = os.Setenv("STORAGE_LOCAL_PATH", filepath.Dir(exePath))
	}

	myApp := app.NewApp(
		app.Option{
			Name: "w7-rangine-go-skeleton",
		},
	)
	slog.Debug("config", "env", facade.GetConfig().GetString("app.env"))
	slog.Debug("config", "storage", storage.Local{}.GetStorageLocalPath())
	slog.Debug("config", "db", facade.GetConfig().GetString("database.default.db_name"))

	// 业务中需要使用 http server，这里需要先实例化
	httpServer := new(http.Provider).Register(myApp.GetConfig(), myApp.GetConsole(), myApp.GetServerManager()).Export()
	// 注册一些全局中间件，路由或是其它一些全局操作
	httpServer.Use(func(context *gin.Context) {
		slog.Info("runtime info", "goroutine", runtime.NumGoroutine(), "client total", ws.GetCollect().Total(), "progress total", ws.GetCollect().ProgressTotal())
	}, middleware.GetPanicHandlerMiddleware())
	// 全局登录判断
	httpServer.Use(common2.AuthMiddleware{}.Process, common2.CacheMiddleware{}.Process)
	httpServer.RegisterRouters(
		func(engine *gin.Engine) {
			subFs, _ := fs.Sub(Asset, "asset/static")
			engine.StaticFS("/dpanel/static/asset", http2.FS(subFs))
			engine.StaticFileFS("/favicon.ico", "icon.jpg", http2.FS(subFs))
			engine.NoRoute(func(http *gin.Context) {
				indexHtml, _ := Asset.ReadFile("asset/static/index.html")
				http.Data(http2.StatusOK, "text/html; charset=UTF-8", indexHtml)
				return
			})
		},
	)

	db, err := facade.GetDbFactory().Channel("default")
	if err != nil {
		panic(err)
	}
	dao.SetDefault(db)

	runEnvType := facade.GetConfig().GetString("app.env")
	if runEnvType != "debug" {
		// 同步数据库
		err := db.Migrator().AutoMigrate(
			&entity.Event{},
			&entity.Image{},
			&entity.Notice{},
			&entity.Registry{},
			&entity.Setting{},
			&entity.Site{},
			&entity.SiteDomain{},
			&entity.Compose{},
			&entity.Backup{},
			&entity.Store{},
		)
		if err != nil {
			panic(err)
		}
		migrateTableData := []migrate.Updater{
			&migrate.Upgrade20240909{},
		}
		for _, updater := range migrateTableData {
			if version.CompareSimple(updater.Version(), myApp.GetConfig().GetString("app.version")) == -1 {
				continue
			}
			slog.Debug("main", "migrate", updater.Version())
			err = updater.Upgrade()
			if err != nil {
				slog.Debug("main", "migrate", err)
			}
		}

		registryRow, _ := dao.Registry.Where(dao.Registry.ServerAddress.Eq("docker.io")).First()
		if registryRow == nil {
			_ = dao.Registry.Create(&entity.Registry{
				Title:         "Docker Hub",
				ServerAddress: "docker.io",
				Setting: &accessor.RegistrySettingOption{
					Username: "anonymous",
					Proxy:    []string{},
				},
			})
		}

		// 初始化挂载目录
		initPath := []string{
			"storage",
			"backup",
			"compose",
		}
		if runEnvType == "production" {
			initPath = append(initPath,
				"nginx/default_host",
				"nginx/proxy_host",
				"nginx/redirection_host",
				"nginx/dead_host",
				"nginx/temp",
				"acme",
				"cert",
				"compose",
			)
		}
		for _, path := range initPath {
			err = os.MkdirAll(storage.Local{}.GetStorageLocalPath()+"/"+path, os.ModePerm)
			if err != nil {
				panic(err.Error())
			}
		}
	}

	// 注册资源
	_ = facade.GetContainer().NamedSingleton("asset", func() embed.FS {
		return Asset
	})

	new(family.Provider).Register(httpServer, facade.GetConsole())
	new(common.Provider).Register(httpServer)
	new(application.Provider).Register(httpServer)
	new(ctrl.Provider).Register(facade.GetConsole())

	myApp.RunConsole()
}
