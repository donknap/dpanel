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
	"github.com/donknap/dpanel/common/migrate"
	"github.com/donknap/dpanel/common/service/storage"
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
	"strings"
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
		os.Setenv("STORAGE_LOCAL_PATH", filepath.Dir(exePath))
	}

	app := app.NewApp(
		app.Option{
			Name: "w7-rangine-go-skeleton",
		},
	)
	slog.Debug("config", "env", facade.GetConfig().GetString("app.env"))
	slog.Debug("config", "storage", storage.Local{}.GetStorageLocalPath())
	slog.Debug("config", "db", facade.GetConfig().GetString("database.default.db_name"))

	// 业务中需要使用 http server，这里需要先实例化
	httpServer := new(http.Provider).Register(app.GetConfig(), app.GetConsole(), app.GetServerManager()).Export()
	// 注册一些全局中间件，路由或是其它一些全局操作
	httpServer.Use(middleware.GetPanicHandlerMiddleware())
	// 全局登录判断
	httpServer.Use(common2.AuthMiddleware{}.Process)
	httpServer.RegisterRouters(
		func(engine *gin.Engine) {
			subFs, _ := fs.Sub(Asset, "asset/static")
			engine.Use(func(c *gin.Context) {
                            if strings.HasPrefix(c.Request.URL.Path, "/dpanel/static") || c.Request.URL.Path == "/favicon.ico" {
                                c.Header("Cache-Control", "max-age=86400")
                            }
                            c.Next()
                        })
			engine.StaticFS("/dpanel/static", http2.FS(subFs))
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
		)
		if err != nil {
			panic(err)
		}
		migrateTableData := []migrate.Updater{
			&migrate.Upgrade20240909{},
			&migrate.Upgrade20241014{},
		}
		for _, updater := range migrateTableData {
			if version.CompareSimple(updater.Version(), app.GetConfig().GetString("app.version")) == -1 {
				continue
			}
			slog.Debug("main", "migrate", updater.Version())
			err = updater.Upgrade()
			if err != nil {
				slog.Debug("main", "migrate", err)
			}
		}

		// 如果没有管理配置新建一条
		founderSetting, _ := dao.Setting.
			Where(dao.Setting.GroupName.Eq(logic.SettingGroupUser)).
			Where(dao.Setting.Name.Eq(logic.SettingGroupUserFounder)).First()
		if founderSetting == nil {
			var (
				username = "admin"
				password = "admin"
			)
			if os.Getenv("INSTALL_USERNAME") != "" {
				username = os.Getenv("INSTALL_USERNAME")
			}
			if os.Getenv("INSTALL_PASSWORD") != "" {
				password = os.Getenv("INSTALL_PASSWORD")
			}

			_ = dao.Setting.Create(&entity.Setting{
				GroupName: logic.SettingGroupUser,
				Name:      logic.SettingGroupUserFounder,
				Value: &accessor.SettingValueOption{
					Password: logic.User{}.GetMd5Password(password, username),
					Username: username,
				},
			})
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

	// 注册业务 provider，此模块中需要使用 http server 和 console
	new(common.Provider).Register(httpServer)
	new(application.Provider).Register(httpServer)
	app.RunConsole()
}
