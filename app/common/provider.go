package common

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/donknap/dpanel/app/common/http/controller"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/function"
	common "github.com/donknap/dpanel/common/middleware"
	"github.com/donknap/dpanel/common/service/crontab"
	"github.com/donknap/dpanel/common/service/family"
	"github.com/donknap/dpanel/common/types"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	httpserver "github.com/we7coreteam/w7-rangine-go/v2/src/http/server"
)

type Provider struct {
}

func (provider *Provider) Register(httpServer *httpserver.Server) {
	httpServer.RegisterRouters(func(engine *gin.Engine) {
		cors := engine.Group(function.RouterRootApi(), common.CorsMiddleware{}.Process)

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
		feature := new(family.Provider).Feature()
		if !function.InArrayArray(feature, types.FeatureFamilyXk) {
			cors.POST("/common/user/login", controller.User{}.Login)
		}

		if !function.InArrayArray(feature, types.FeatureFamilyPe, types.FeatureFamilyXk) {
			cors.POST("/common/user/oauth/callback", controller.User{}.OauthCallback)
		}

		cors.POST("/common/user/create-founder", controller.User{}.CreateFounder)
		cors.POST("/common/user/login-info", controller.User{}.LoginInfo)
		cors.POST("/common/user/get-user-info", controller.User{}.GetUserInfo)

		// 配置
		cors.POST("/common/setting/founder", controller.Setting{}.Founder)
		cors.POST("/common/setting/get-setting", controller.Setting{}.GetSetting)
		cors.POST("/common/setting/save-config", controller.Setting{}.SaveConfig)
		cors.POST("/common/setting/delete", controller.Setting{}.Delete)
		cors.POST("/common/setting/notification-email-test", controller.Home{}.NotificationEmailTest)

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
		cors.POST("/common/explorer/get-user-list", controller.Explorer{}.GetUserList)
		cors.POST("/common/explorer/get-content", controller.Explorer{}.GetContent)
		cors.POST("/common/explorer/get-file-stat", controller.Explorer{}.GetFileStat)
		cors.POST("/common/explorer/import", controller.Explorer{}.Import)
		cors.POST("/common/explorer/export", controller.Explorer{}.Export)
		cors.POST("/common/explorer/import-file-content", controller.Explorer{}.ImportFileContent)
		cors.POST("/common/explorer/unzip", controller.Explorer{}.Unzip)
		cors.POST("/common/explorer/delete", controller.Explorer{}.Delete)
		cors.POST("/common/explorer/chmod", controller.Explorer{}.Chmod)
		cors.POST("/common/explorer/mkdir", controller.Explorer{}.MkDir)
		cors.POST("/common/explorer/copy", controller.Explorer{}.Copy)
	})

	httpServer.RegisterRouters(func(engine *gin.Engine) {
		wsCors := engine.Group(function.RouterRootWs(), common.CorsMiddleware{}.Process)

		wsCors.GET("/common/notice", controller.Home{}.WsNotice)
		wsCors.GET("/common/console/container/:id", controller.Home{}.WsContainerConsole)
		wsCors.GET("/common/console/host/:name", controller.Home{}.WsHostConsole)
	})

	// 启动时，初始化计划任务
	if cronList, err := dao.Cron.Order(dao.Cron.ID.Desc()).Find(); err == nil {
		for _, task := range cronList {
			if task.Setting.Disable {
				continue
			}
			jobIds, err := logic.Cron{}.AddJob(task)
			if err != nil {
				task.Setting.NextRunTime = make([]time.Time, 0)
				task.Setting.JobIds = make([]cron.EntryID, 0)
				slog.Debug("init crontab task error", "error", err.Error())
			} else {
				task.Setting.NextRunTime = crontab.Wrapper.GetNextRunTime(jobIds...)
				task.Setting.JobIds = jobIds
			}
			_ = dao.Cron.Save(task)
		}
	}
}
