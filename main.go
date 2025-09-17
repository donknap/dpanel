package main

import (
	"bytes"
	"context"
	"embed"
	_ "embed"
	"fmt"
	"github.com/docker/docker/client"
	"github.com/donknap/dpanel/app/application"
	"github.com/donknap/dpanel/app/common"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/app/ctrl"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	common2 "github.com/donknap/dpanel/common/middleware"
	"github.com/donknap/dpanel/common/migrate"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/exec/local"
	"github.com/donknap/dpanel/common/service/family"
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
	"io/fs"
	"log/slog"
	http2 "net/http"
	"os"
	"path/filepath"
	"time"
)

var (
	//go:embed config.yaml
	ConfigFile []byte
	//go:embed asset
	Asset         embed.FS
	DPanelVersion = ""
)

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

	slog.Debug("config", "env", facade.GetConfig().GetString("app.env"))
	slog.Debug("config", "version", DPanelVersion)
	slog.Debug("config", "storage", storage.Local{}.GetStorageLocalPath())
	slog.Debug("config", "db", facade.GetConfig().GetString("database.default.db_name"))
	if DPanelVersion != "" {
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
		err = initDefaultDocker()
		if err != nil {
			panic(err)
		}

		if isDebug() {
			storage.Cache.Set(storage.CacheKeyCommonServerStartTime, time.Date(2024, 6, 5, 0, 0, 0, 0, time.UTC), cache.DefaultExpiration)
		} else {
			storage.Cache.Set(storage.CacheKeyCommonServerStartTime, time.Now(), cache.DefaultExpiration)
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
				gzipMiddleware.StaticFS("/dpanel/static/asset", http2.FS(subFs))
				engine.StaticFileFS("/favicon.ico", "dpanel.ico", http2.FS(subFs))
				engine.NoRoute(func(http *gin.Context) {
					slog.Debug("http route not found", "uri", http.Request.URL.String())
					indexHtml, _ := Asset.ReadFile("asset/static/index.html")
					http.Data(http2.StatusOK, "text/html; charset=UTF-8", indexHtml)
					return
				})
				engine.Static("/dpanel/static/image", filepath.Join(storage.Local{}.GetSaveRootPath(), "image"))
			},
		)

		new(family.Provider).Register(httpServer)
		new(common.Provider).Register(httpServer)
		new(application.Provider).Register(httpServer)
	}

	new(ctrl.Provider).Register(facade.GetConsole())
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
		"nginx/default_host",
		"nginx/proxy_host",
		"nginx/redirection_host",
		"nginx/dead_host",
		"nginx/temp",
		"script",
		"storage",
		"store",
		"logs",
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
	// 用户可以自行挂载证书，如果没有则自动生成
	// 证书用于验证 ssh 登录以及 jwt 签名
	homeDir, _ := os.UserHomeDir()
	userRsaIdFiles := []string{
		filepath.Join(homeDir, ".ssh", define.DefaultIdPubFile),
		filepath.Join(homeDir, ".ssh", define.DefaultIdKeyFile),
	}

	rsaIdFiles := []string{
		filepath.Join(storage.Local{}.GetCertRsaPath(), define.DefaultIdPubFile),
		filepath.Join(storage.Local{}.GetCertRsaPath(), define.DefaultIdKeyFile),
	}

	if function.FileExists(userRsaIdFiles...) {
		// 如果系统已经存在了 id_rsa 表示当前非容器内
		// 将系统的 rsa 文件复制过来方便后续使用
		for _, file := range rsaIdFiles {
			_ = os.Remove(file)
		}
		err := function.CopyFile(storage.Local{}.GetCertRsaPath(), userRsaIdFiles...)
		if err != nil {
			return err
		}
		return nil
	}

	if !function.FileExists(rsaIdFiles...) {
		for _, file := range rsaIdFiles {
			_ = os.Remove(file)
		}
		_, err := local.QuickRun(fmt.Sprintf(
			`ssh-keygen -t rsa -b 4096 -f %s -N "" -C "%s@%s"`,
			filepath.Join(storage.Local{}.GetCertRsaPath(), define.DefaultIdKeyFile),
			docker.BuilderAuthor,
			docker.BuildWebSite,
		))
		if err != nil {
			return err
		}
	}

	err := function.CopyDir(filepath.Join(homeDir, ".ssh"), storage.Local{}.GetCertRsaPath())
	if err != nil {
		return err
	}

	for _, file := range userRsaIdFiles {
		_ = os.Chmod(file, 0600)
	}

	return nil
}

func initDefaultDocker() error {
	// 当前如果有连接，则添加一条docker环境数据
	defaultDockerHost := client.DefaultDockerHost
	if e := os.Getenv(client.EnvOverrideHost); e != "" {
		defaultDockerHost = e
	}
	var defaultDockerEnv *docker.Client
	var err error

	if v, err := (logic.DockerEnv{}).GetDefaultEnv(); err == nil {
		defaultDockerEnv = v
	} else {
		defaultDockerEnv = &docker.Client{
			Name:    docker.DefaultClientName,
			Title:   docker.DefaultClientName,
			Address: defaultDockerHost,
			Default: true,
		}
	}

	docker.Sdk, err = docker.NewBuilderWithDockerEnv(defaultDockerEnv)
	if err != nil {
		// 如果无法连接，创建一个默认 docker.sdk 期待用户在面板中修改连接配置
		docker.Sdk, err = docker.NewBuilder(docker.WithAddress(defaultDockerEnv.Address), docker.WithDockerEnv(defaultDockerEnv))
		return nil
	}
	// 使用超时上下文，避免 docker 连接地址时间过长卡死程序
	ctx, _ := context.WithTimeout(context.Background(), docker.ConnectDockerServerTimeout)
	if dockerInfo, err := docker.Sdk.Client.Info(ctx); err == nil {
		go logic.NewEventLogin().MonitorLoop()

		defaultDockerEnv.DockerInfo = &docker.ClientDockerInfo{
			Name: dockerInfo.Name,
			ID:   dockerInfo.ID,
		}

		// 面板信息总是从默认环境中获取
		if info, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, facade.GetConfig().GetString("app.name")); err == nil {
			info.ExecIDs = make([]string, 0)
			_ = logic.Setting{}.Save(&entity.Setting{
				GroupName: logic.SettingGroupSetting,
				Name:      logic.SettingGroupSettingDPanelInfo,
				Value: &accessor.SettingValueOption{
					DPanelInfo: &info,
				},
			})
		} else {
			_ = logic.Setting{}.Delete(logic.SettingGroupSetting, logic.SettingGroupSettingDPanelInfo)
			slog.Warn("init dpanel info", "name", facade.GetConfig().GetString("app.name"), "error", err)
		}
	} else {
		// 获取信息失败，期待用户在面板中修改默认连接的配置
		slog.Warn("connect default docker server failed", "name", defaultDockerEnv.Name, "address", defaultDockerEnv.Address, "error", err)
		defaultDockerEnv.DockerInfo = nil
		docker.Sdk.Close()
	}
	logic.DockerEnv{}.UpdateEnv(defaultDockerEnv)
	return nil
}
