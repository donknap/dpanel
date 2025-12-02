package events

import (
	"fmt"
	"log/slog"

	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/donknap/dpanel/common/types/event"
	"github.com/patrickmn/go-cache"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

type Docker struct {
}

func (self Docker) Start(e event.DockerPayload) {
	if e.DockerEnv == nil {
		return
	}
	storage.Cache.Delete(fmt.Sprintf(storage.CacheKeyDockerStatus, e.DockerEnv.Name))

	if e.DockerEnv.Name != define.DockerDefaultClientName {
		return
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

func (self Docker) Stop(e event.DockerPayload) {
	if e.DockerEnv == nil {
		return
	}
	storage.Cache.Set(fmt.Sprintf(storage.CacheKeyDockerStatus, e.DockerEnv.Name), types.DockerStatus{
		Available: false,
		Message:   e.Error.Error(),
	}, cache.DefaultExpiration)
}

func (self Docker) Message(e event.DockerMessagePayload) {
	if e.Type == "container/stop" {
		fmt.Printf("Message %v \n", e.Message[0])
	}
}
