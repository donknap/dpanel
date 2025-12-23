package events

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/crontab"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/donknap/dpanel/common/types/event"
	"github.com/patrickmn/go-cache"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

var (
	dataPool = make([]*entity.Event, 0)
	mu       = sync.Mutex{}
	ticker   = time.NewTicker(time.Second * 10)
)

func init() {
	go func() {
		for {
			<-ticker.C
			commit()
		}
	}()
}

func commit() {
	if len(dataPool) == 0 {
		return
	}
	slog.Debug("Event monitor commit start", "length", len(dataPool))

	mu.Lock()
	defer mu.Unlock()

	db, err := facade.GetDbFactory().Channel("default")
	if err != nil {
		slog.Debug("Event monitor commit", "err", err)
		return
	}

	err = db.CreateInBatches(dataPool, len(dataPool)).Error
	if err != nil {
		slog.Debug("Event monitor commit", "len", len(dataPool), "err", err)
		return
	}
	dataPool = []*entity.Event{}
	return
}

type Docker struct {
}

func (self Docker) Daemon(e event.DockerDaemonPayload) {
	if e.DockerEnv == nil {
		return
	}
	dockerStatusCacheKey := fmt.Sprintf(storage.CacheKeyDockerStatus, e.DockerEnv.Name)
	// 如果有错误记录缓存返回
	if !e.Status.Available {
		storage.Cache.Set(dockerStatusCacheKey, e.Status, cache.DefaultExpiration)
		return
	}

	storage.Cache.Delete(dockerStatusCacheKey)

	if e.DockerEnv.Name != define.DockerDefaultClientName {
		return
	}
	// 默认环境的配置可能会被更改，重新获取最新的配置再填充 DPanel 容器数据
	if v, err := (logic.Env{}).GetDefaultEnv(); err == nil {
		e.DockerEnv = v
	}

	sdk, err := docker.NewClientWithDockerEnv(e.DockerEnv)
	if err != nil {
		return
	}
	defer func() {
		sdk.Close()
	}()
	if dockerInfo, err := sdk.Client.Info(sdk.Ctx); err == nil {
		e.DockerEnv.DockerInfo = &types.DockerInfo{
			ID:            dockerInfo.ID,
			Name:          dockerInfo.Name,
			KernelVersion: dockerInfo.KernelVersion,
			Architecture:  dockerInfo.Architecture,
			OSType:        dockerInfo.OSType,
			InDPanel:      true,
		}
		// 面板信息总是从默认环境中获取
		if info, err := sdk.Client.ContainerInspect(sdk.Ctx, facade.GetConfig().GetString("app.name")); err == nil {
			info.ExecIDs = make([]string, 0)
			_ = logic.Setting{}.Save(&entity.Setting{
				GroupName: logic.SettingGroupSetting,
				Name:      logic.SettingGroupSettingDPanelInfo,
				Value: &accessor.SettingValueOption{
					DPanelInfo: &info,
				},
			})
		} else {
			e.DockerEnv.DockerInfo.InDPanel = false
			_ = logic.Setting{}.Delete(logic.SettingGroupSetting, logic.SettingGroupSettingDPanelInfo)
			slog.Warn("init dpanel info", "name", facade.GetConfig().GetString("app.name"), "error", err)
		}
		logic.Env{}.UpdateEnv(e.DockerEnv)
	}
}

func (self Docker) Message(e event.DockerMessagePayload) {
	if !function.IsEmptyArray(e.Message) {
		mu.Lock()
		defer mu.Unlock()
		dataPool = append(dataPool, &entity.Event{
			Type:      e.Type,
			Action:    e.Action,
			Message:   strings.Join(e.Message, " "),
			CreatedAt: time.UnixMilli(e.Time / 1000000).Format("2006-01-02 15:04:05.000"),
		})
	}

	if function.InArray([]string{
		define.DockerEventContainerDestroy, define.DockerEventContainerCreate,
		define.DockerEventContainerDie, define.DockerEventContainerStart,
	}, e.Action) {
		crontab.Client.RunByEvent(e.Action, []types.EnvItem{
			types.NewEnvItemFromKV("DP_DOCKER_ENV_NAME", e.Type),
			types.NewEnvItemFromKV("DP_CONTAINER_NAME", e.Message[0]),
		})
	}
}
