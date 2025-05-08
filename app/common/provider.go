package common

import (
	"context"
	"github.com/docker/docker/client"
	"github.com/donknap/dpanel/app/common/http/controller"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	common "github.com/donknap/dpanel/common/middleware"
	"github.com/donknap/dpanel/common/service/crontab"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	http_server "github.com/we7coreteam/w7-rangine-go/v2/src/http/server"
	"log/slog"
	"net/http"
	"os"
	"time"
)

type Provider struct {
}

func (provider *Provider) Register(httpServer *http_server.Server) {
	httpServer.RegisterRouters(func(engine *gin.Engine) {
		cors := engine.Group("/api", common.CorsMiddleware{}.Process)

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
		cors.POST("/common/user/create-founder", controller.User{}.CreateFounder)
		cors.POST("/common/user/login-info", controller.User{}.LoginInfo)
		cors.POST("/common/user/get-user-info", controller.User{}.GetUserInfo)
		cors.POST("/common/user/save-theme-config", controller.User{}.SaveThemeConfig)

		// 配置
		cors.POST("/common/setting/founder", controller.Setting{}.Founder)
		cors.POST("/common/setting/email", controller.Setting{}.Email)
		cors.POST("/common/setting/email-test", controller.Setting{}.EmailTest)
		cors.POST("/common/setting/get-setting", controller.Setting{}.GetSetting)

		cors.POST("/common/home/info", controller.Home{}.Info)
		cors.POST("/common/home/check-new-version", controller.Home{}.CheckNewVersion)
		cors.POST("/common/home/usage", controller.Home{}.Usage)
		cors.POST("/common/home/upgrade-script", controller.Home{}.UpgradeScript)
		cors.POST("/common/home/get-stat-list", controller.Home{}.GetStatList)

		// 环境管理
		cors.POST("/common/env/get-list", controller.Env{}.GetList)
		cors.POST("/common/env/create", controller.Env{}.Create)
		cors.POST("/common/env/switch", controller.Env{}.Switch)
		cors.POST("/common/env/delete", controller.Env{}.Delete)
		cors.POST("/common/env/get-detail", controller.Env{}.GetDetail)

		// 应用商店
		cors.POST("/common/store/create", controller.Store{}.Create)
		cors.POST("/common/store/get-list", controller.Store{}.GetList)
		cors.POST("/common/store/delete", controller.Store{}.Delete)
		cors.POST("/common/store/sync", controller.Store{}.Sync)
		cors.POST("/common/store/deploy", controller.Store{}.Deploy)

		engine.StaticFS("/dpanel/static/store/file", http.FS(logic.StoreLogoFileSystem{}))

		// 计划任务
		cors.POST("/common/cron/create", controller.Cron{}.Create)
		cors.POST("/common/cron/get-list", controller.Cron{}.GetList)
		cors.POST("/common/cron/get-detail", controller.Cron{}.GetDetail)
		cors.POST("/common/cron/delete", controller.Cron{}.Delete)
		cors.POST("/common/cron/run-once", controller.Cron{}.RunOnce)
		cors.POST("/common/cron/get-log-list", controller.Cron{}.GetLogList)
		cors.POST("/common/cron/prune-log", controller.Cron{}.PruneLog)
		cors.POST("/common/cron/template", controller.Cron{}.Template)

		// 标签及分组
		cors.POST("/common/tag/create", controller.Tag{}.Create)
		cors.POST("/common/tag/get-list", controller.Tag{}.GetList)
		cors.POST("/common/tag/delete", controller.Tag{}.Delete)

		// 文件相关
		cors.POST("/common/explorer/get-path-list", controller.Explorer{}.GetPathList)
	})

	httpServer.RegisterRouters(func(engine *gin.Engine) {
		wsCors := engine.Group("/ws/", common.CorsMiddleware{}.Process)

		wsCors.GET("/common/notice", controller.Home{}.WsNotice)
		wsCors.GET("/common/console/container/:id", controller.Home{}.WsContainerConsole)
		wsCors.GET("/common/console/host/:name", controller.Home{}.WsHostConsole)
	})

	// 当前如果有连接，则添加一条docker环境数据
	defaultDockerHost := client.DefaultDockerHost
	if e := os.Getenv(client.EnvOverrideHost); e != "" {
		defaultDockerHost = e
	}

	defaultDockerEnv, err := logic.DockerEnv{}.GetEnvByName(docker.DefaultClientName)
	if err != nil {
		defaultDockerEnv = &docker.Client{
			Name:    docker.DefaultClientName,
			Title:   docker.DefaultClientName,
			Address: defaultDockerHost,
			Default: true,
		}
		logic.DockerEnv{}.UpdateEnv(defaultDockerEnv)
	}

	options := []docker.Option{
		docker.WithName(defaultDockerEnv.Name),
		docker.WithAddress(defaultDockerEnv.Address),
	}
	if defaultDockerEnv.EnableTLS {
		options = append(options, docker.WithTLS(defaultDockerEnv.TlsCa, defaultDockerEnv.TlsCert, defaultDockerEnv.TlsKey))
	}
	docker.Sdk, err = docker.NewBuilder(options...)

	// 使用超时上下文，避免 docker 连接地址时间过长卡死程序
	ctx, _ := context.WithTimeout(context.Background(), docker.ConnectDockerServerTimeout)
	defaultDockerInfo, err := docker.Sdk.Client.Info(ctx)

	if err == nil {
		defaultDockerEnv.DockerInfo = &docker.ClientDockerInfo{
			Name: defaultDockerInfo.Name,
			ID:   defaultDockerInfo.ID,
		}
		logic.DockerEnv{}.UpdateEnv(defaultDockerEnv)
		// 获取面板信息
		if info, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, facade.GetConfig().GetString("app.name")); err == nil {
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
		go logic.EventLogic{}.MonitorLoop()
	} else {
		slog.Warn("connect default docker server failed", "name", defaultDockerEnv.Name, "address", defaultDockerEnv.Address, "error", err)
		defaultDockerEnv.DockerInfo = nil
		logic.DockerEnv{}.UpdateEnv(defaultDockerEnv)
	}

	// 启动时，初始化计划任务
	if cronList, err := dao.Cron.Order(dao.Cron.ID.Desc()).Find(); err == nil {
		for _, task := range cronList {
			jobIds, err := logic.Cron{}.AddJob(task)
			if err != nil {
				task.Setting.NextRunTime = make([]time.Time, 0)
				task.Setting.JobIds = make([]cron.EntryID, 0)
				slog.Debug("init crontab task error", "error", err.Error())
			} else {
				task.Setting.NextRunTime = crontab.Wrapper.GetNextRunTime(jobIds...)
				task.Setting.JobIds = jobIds
			}
			_, _ = dao.Cron.Updates(task)
		}
	}
}
