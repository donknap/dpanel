package main

import (
	"bytes"
	"embed"
	_ "embed"
	"fmt"
	"io/fs"
	"log/slog"
	http2 "net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/client"
	"github.com/donknap/dpanel/app/application"
	"github.com/donknap/dpanel/app/common"
	"github.com/donknap/dpanel/app/common/http/controller"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/app/ctrl"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	common2 "github.com/donknap/dpanel/common/middleware"
	"github.com/donknap/dpanel/common/migrate"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/exec/local"
	"github.com/donknap/dpanel/common/service/family"
	fs2 "github.com/donknap/dpanel/common/service/fs"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/patrickmn/go-cache"
	"github.com/spf13/viper"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	app "github.com/we7coreteam/w7-rangine-go/v2/src"
	"github.com/we7coreteam/w7-rangine-go/v2/src/core/helper"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/middleware"
)

var (
	//go:embed config.yaml
	ConfigFile []byte
	//go:embed asset
	Asset embed.FS
)

const dpanelBanner = `DPanel is a lightweight container management panel; visit https://deepanel.com for more information.`

var DPanelVersion string

func main() {
	// 兼容没有配置存储目录的情况
	if os.Getenv("STORAGE_LOCAL_PATH") == "" {
		exePath, _ := os.Executable()
		_ = os.Setenv("STORAGE_LOCAL_PATH", filepath.Dir(exePath))
	}

	storagePath := os.Getenv("STORAGE_LOCAL_PATH")
	storagePathStat, err := os.Stat(storagePath)
	if err == nil && !storagePathStat.IsDir() {
		panic(fmt.Sprintf("%s must be a directory", storagePathStat.Name()))
	}
	if err != nil {
		_ = os.MkdirAll(storagePath, os.ModePerm)
	}

	myApp := app.NewApp(
		app.Option{
			Name:    "w7-rangine-go-skeleton",
			Version: DPanelVersion,
			DefaultConfigLoader: func(config *viper.Viper) {
				config.SetConfigType("yaml")
				err := config.MergeConfig(bytes.NewReader(helper.ParseConfigContentEnv(ConfigFile)))
				if err != nil {
					panic(err)
				}
			},
		},
	)
	if DPanelVersion == "" {
		DPanelVersion = facade.GetConfig().GetString("app.version")
	} else {
		facade.GetConfig().Set("app.version", DPanelVersion)
	}

	// 注册数据库
	db, err := facade.GetDbFactory().Channel("default")
	if err != nil {
		panic(err)
	}
	dao.SetDefault(db)

	// 注册资源
	_ = facade.GetContainer().NamedSingleton("asset", func() embed.FS {
		return Asset
	})

	if isAppServer() {
		slog.Warn(dpanelBanner)
		slog.Debug("config", "env", facade.GetConfig().GetString("app.env"))
		slog.Debug("config", "version", DPanelVersion)
		slog.Debug("config", "storage", storage.Local{}.GetStorageLocalPath())

		err = initDb()
		if err != nil {
			panic(err)
		}
		err = initPath()
		if err != nil {
			panic(err)
		}
		err = initRSA()
		if err != nil {
			panic(err)
		}
		err = initDocker()
		if err != nil {
			panic(err)
		}

		if isDebug() {
			storage.Cache.Set(storage.CacheKeyCommonServerStartTime, time.Date(2024, 6, 5, 0, 0, 0, 0, time.UTC), cache.DefaultExpiration)
		} else {
			storage.Cache.Set(storage.CacheKeyCommonServerStartTime, time.Now(), cache.DefaultExpiration)
		}

		if v, err := Asset.ReadDir("asset/static/i18n"); err == nil {
			storage.Cache.Set(storage.CacheKeySettingLocale, function.PluckArrayWalk(v, func(item fs.DirEntry) (string, bool) {
				return strings.TrimSuffix(item.Name(), filepath.Ext(item.Name())), true
			}), cache.DefaultExpiration)
		}

		// 业务中需要使用 http server，这里需要先实例化
		httpServer := new(http.Provider).Register(myApp.GetConfig(), myApp.GetConsole(), myApp.GetServerManager()).Export()
		// 注册一些全局中间件，路由或是其它一些全局操作
		httpServer.Use(common2.DebugMiddleware{}.Process, middleware.GetPanicHandlerMiddleware())
		// 注册 family 中间件
		if v := (family.Provider{}).Middleware(); v != nil {
			httpServer.Use(v...)
		}
		httpServer.Use(common2.AuthMiddleware{}.Process, common2.CacheMiddleware{}.Process)
		httpServer.RegisterRouters(
			func(engine *gin.Engine) {
				subFs, _ := fs.Sub(Asset, "asset/static")
				gzipMiddleware := engine.Use(gzip.Gzip(gzip.DefaultCompression))
				gzipMiddleware.StaticFS(function.RouterUri("/dpanel/static/asset"), fs2.NewHttpFs(subFs))

				engine.StaticFileFS(function.RouterUri("/favicon.ico"), "/img/dpanel.ico", http2.FS(subFs))
				engine.Static(function.RouterUri("/dpanel/static/image"), filepath.Join(storage.Local{}.GetSaveRootPath(), "image"))

				engine.NoRoute(func(http *gin.Context) {
					http.Set("asset", Asset)
					controller.Home{}.Index(http)
					return
				})
			},
		)

		new(family.Provider).Register(httpServer)
		new(common.Provider).Register(httpServer)
		new(application.Provider).Register(httpServer)
	}

	new(ctrl.Provider).Command(facade.GetConsole())
	myApp.RunConsole()
}

func isAppServer() bool {
	return function.InArray(os.Args, "server:start")
}

func isDebug() bool {
	return facade.GetConfig().GetString("app.env") == "debug"
}

func initDb() error {
	if isDebug() {
		return nil
	}
	db, err := facade.GetDbFactory().Channel("default")
	if err != nil {
		return err
	}
	// 同步数据库
	err = db.Migrator().AutoMigrate(
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
		&entity.Cron{},
		&entity.CronLog{},
	)
	if err != nil {
		return err
	}
	migrateTableData := []migrate.Updater{
		&migrate.Upgrade20240909{},
		&migrate.Upgrade20250106{},
		&migrate.Upgrade20250113{},
		&migrate.Upgrade20250401{},
		&migrate.Upgrade20250521{},
	}
	for _, updater := range migrateTableData {
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
	return nil
}

func initPath() error {
	runEnvType := facade.GetConfig().GetString("app.env")
	if runEnvType == "debug" {
		return nil
	}
	// 初始化挂载目录
	initPathList := []string{
		"acme",
		"backup",
		"cert",
		"cert/rsa",
		"cert/docker",
		"compose",
		"nginx/extra_host",
		"nginx/proxy_host",
		"nginx/temp",
		"script",
		"storage",
		"store",
		"logs",
		"sock",
	}
	for _, path := range initPathList {
		realPath := storage.Local{}.GetStorageLocalPath() + "/" + path
		if _, err := os.Lstat(realPath); err != nil {
			err = os.MkdirAll(realPath, os.ModePerm)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func initRSA() error {
	// 用户可以自行配置证书的位置，如果没有则自动生成
	// 证书用于 jwt 签名及其它密码加密
	rsaIdFiles := []string{
		facade.GetConfig().GetString("system.rsa.pub"),
		facade.GetConfig().GetString("system.rsa.key"),
	}

	if !function.FileExists(rsaIdFiles...) {
		for _, file := range rsaIdFiles {
			_ = os.Remove(file)
		}
		_, err := local.QuickRun("ssh-keygen",
			"-t", "rsa",
			"-b", "4096",
			"-f", rsaIdFiles[1],
			"-N", "",
			"-C", define.PanelAuthor+"@"+define.PanelWebSite)
		if err != nil {
			return err
		}
	}

	contents := function.PluckArrayWalk(rsaIdFiles, func(p string) ([]byte, bool) {
		c, err := os.ReadFile(p)
		return c, err == nil
	})

	// 将 rsa 加载内存中，方便使用
	storage.Cache.Set(storage.CacheKeyRsaPub, contents[0], cache.DefaultExpiration)
	storage.Cache.Set(storage.CacheKeyRsaKey, contents[1], cache.DefaultExpiration)

	return nil
}

func initDocker() error {
	// 当前如果有连接，则添加一条docker环境数据
	defaultDockerHost := client.DefaultDockerHost
	if e := os.Getenv(client.EnvOverrideHost); e != "" {
		defaultDockerHost = e
	}
	var defaultDockerEnv *types.DockerEnv
	if v, err := (logic.Env{}).GetDefaultEnv(); err == nil {
		defaultDockerEnv = v
	} else {
		defaultDockerEnv = &types.DockerEnv{
			Name:    define.DockerDefaultClientName,
			Title:   define.DockerDefaultClientName,
			Address: defaultDockerHost,
			Default: true,
		}
		logic.Env{}.UpdateEnv(defaultDockerEnv)
	}
	if dockerClient, err := docker.NewClientWithDockerEnv(defaultDockerEnv, docker.WithSockProxy()); err == nil {
		docker.Sdk = dockerClient
	} else {
		slog.Debug("init docker", "error", err, "env", defaultDockerEnv)
	}

	dockerEnvList := make(map[string]*types.DockerEnv)
	logic.Setting{}.GetByKey(logic.SettingGroupSetting, logic.SettingGroupSettingDocker, &dockerEnvList)
	for _, env := range dockerEnvList {
		notice.Monitor.Join(env)
	}
	return nil
}
