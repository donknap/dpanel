package events

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"sync"

	types3 "github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/docker/api/types/container"
	dockerEvents "github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/network"
	logic2 "github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/crontab"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/exec"
	"github.com/donknap/dpanel/common/service/exec/local"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/plugin"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/service/ws"
	types2 "github.com/donknap/dpanel/common/types"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/donknap/dpanel/common/types/event"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/patrickmn/go-cache"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

const maxDockerMessageSize = 100

var (
	dockerMessageFilterContainerList = []string{
		plugin.ExplorerName,
		plugin.MonitorName,
	}
	dockerMessageLevelMap = map[dockerEvents.Action]types2.LogLevel{
		dockerEvents.ActionOOM:          types2.LogLevelError,
		dockerEvents.ActionDie:          types2.LogLevelWarning,
		dockerEvents.ActionDestroy:      types2.LogLevelWarning,
		dockerEvents.ActionRemove:       types2.LogLevelWarning,
		dockerEvents.ActionDelete:       types2.LogLevelWarning,
		dockerEvents.ActionPrune:        types2.LogLevelWarning,
		dockerEvents.ActionStop:         types2.LogLevelWarning,
		dockerEvents.ActionRestart:      types2.LogLevelWarning,
		dockerEvents.ActionKill:         types2.LogLevelWarning,
		dockerEvents.ActionPause:        types2.LogLevelWarning,
		dockerEvents.ActionUnmount:      types2.LogLevelWarning,
		dockerEvents.ActionDisconnect:   types2.LogLevelWarning,
		dockerEvents.ActionDisable:      types2.LogLevelWarning,
		dockerEvents.ActionHealthStatus: types2.LogLevelDebug,
		dockerEvents.ActionAttach:       types2.LogLevelDebug,
		dockerEvents.ActionDetach:       types2.LogLevelDebug,
		dockerEvents.ActionResize:       types2.LogLevelDebug,
		dockerEvents.ActionTop:          types2.LogLevelDebug,
		dockerEvents.ActionExecCreate:   types2.LogLevelDebug,
		dockerEvents.ActionExecStart:    types2.LogLevelDebug,
		dockerEvents.ActionExecDie:      types2.LogLevelDebug,
		dockerEvents.ActionExecDetach:   types2.LogLevelDebug,
	}
	dockerMessageMu = sync.Mutex{}
)

type Docker struct {
}

func (self Docker) Daemon(e event.DockerDaemonPayload) {
	slog.Debug("docker daemon/event start", "info", e)

	if e.DockerEnvName == "" {
		slog.Warn("docker daemon/event", "error", "docker env is nil")
		return
	}

	dockerStatusCacheKey := fmt.Sprintf(storage.CacheKeyDockerStatus, e.DockerEnvName)
	storage.Cache.Set(dockerStatusCacheKey, e.Status, cache.DefaultExpiration)

	slog.Debug("docker daemon/event status", "cacheKey", dockerStatusCacheKey, "status", e.Status)

	// 连接失败时保留错误状态，供环境列表展示断连原因
	if !e.Status.Available {
		return
	}

	// 默认环境的配置可能会被更改，重新获取最新的配置再填充 DPanel 容器数据
	dockerEnv, err := logic.Env{}.GetEnvByName(e.DockerEnvName)
	if err != nil {
		slog.Warn("docker daemon/event env not found", "error", err)
		return
	}
	// 连接成功后，并且判断一下是否是当前连接, 如果当前连接不通，就重置一下
	if docker.Sdk.Name == dockerEnv.Name {
		if _, err := docker.Sdk.Client.Ping(docker.Sdk.Ctx); err != nil {
			docker.Sdk.Close()
			if v, err := docker.NewClientWithDockerEnv(dockerEnv, docker.WithSockProxy()); err == nil {
				docker.Sdk = v
				slog.Debug("docker daemon/event update docker.Sdk", "address", v.Client.DaemonHost())
			}
		}
		// 当前的状态有变化，强制前端重新刷新一下状态
		ws.PushEvent(ws.MessageTypeEventRefreshDockerEnv, gin.H{
			"name": docker.Sdk.Name,
		})
	}

	if dockerEnv.Name != define.DockerDefaultClientName {
		return
	}

	slog.Debug("docker daemon/event update dpanel info", "name", facade.GetConfig().GetString("app.name"))

	sdk, err := docker.NewClientWithDockerEnv(dockerEnv)
	if err != nil {
		return
	}
	defer func() {
		sdk.Close()
	}()

	result := logic.Setting{}.GetDPanelInfo()
	if dockerInfo, err := sdk.Client.Info(sdk.Ctx); err == nil {
		dockerEnv.DockerInfo = &types.DockerInfo{
			ID:              dockerInfo.ID,
			Name:            dockerInfo.Name,
			KernelVersion:   dockerInfo.KernelVersion,
			Architecture:    dockerInfo.Architecture,
			OSType:          dockerInfo.OSType,
			OperatingSystem: dockerInfo.OperatingSystem,
			InDPanel:        true,
		}

		// 面板信息总是从默认环境中获取
		dpanelContainerName := facade.GetConfig().GetString("app.name")
		if function.IsRunInDocker() {
			result.RunIn = types2.DPanelRunInContainer
			if info, err := sdk.Client.ContainerInspect(sdk.Ctx, dpanelContainerName); err == nil {
				info.ExecIDs = make([]string, 0)
				result.ContainerInfo = info
				result.Mount = types.VolumeItem{}
				if v, _, ok := function.PluckArrayItemWalk(info.Mounts, func(item container.MountPoint) bool {
					return item.Destination == "/dpanel"
				}); ok {
					result.Mount = types.VolumeItem{Host: v.Source, Dest: v.Destination, Type: string(v.Type)}
					if v.Type == types3.VolumeTypeVolume {
						result.Mount.Host = v.Name
					}
				}
				result.DataMounts = nil
				if result.Mount.Host != "" && (result.Mount.Type == "bind" || result.Mount.Type == "volume") {
					result.DataMounts = []types.VolumeItem{result.Mount}
					for _, mount := range info.Mounts {
						if !strings.HasPrefix(mount.Destination, "/dpanel/") {
							continue
						}
						item := types.VolumeItem{Host: mount.Source, Dest: mount.Destination, Type: string(mount.Type)}
						if mount.Type == types3.VolumeTypeVolume {
							item.Host = mount.Name
						}
						if item.Host == "" || mount.Type != types3.VolumeTypeBind && mount.Type != types3.VolumeTypeVolume {
							slog.Warn("dpanel data mount is unavailable", "destination", mount.Destination, "type", mount.Type)
							result.DataMounts = nil
							break
						}
						result.DataMounts = append(result.DataMounts, item)
					}
				}
				// 只有在容器才会包含 nginx 功能，如果有网络就自动中入，并重启 nginx
				if _, err := sdk.Client.NetworkInspect(sdk.Ctx, define.DPanelProxyNetworkName, network.InspectOptions{}); err == nil {
					_ = sdk.Client.NetworkConnect(sdk.Ctx, define.DPanelProxyNetworkName, info.ID, &network.EndpointSettings{
						Aliases: []string{
							fmt.Sprintf(define.DPanelNetworkHostName, strings.Trim(info.Name, "/")),
						},
					})
					var nginxErr error
					var cmd exec.Executor
					if facade.GetConfig().Get("app.env") == define.PanelAppEnvStandard {
						err = logic2.Site{}.MakeNginxResolver()
						if err != nil {
							slog.Warn("init nginx make resolver", "error", err)
						}
						_, nginxErr = local.QuickRun("nginx -s reload")
						if nginxErr != nil {
							// 尝试启动 nginx
							if cmd, err = local.New(
								local.WithCommandName("nginx"),
								local.WithArgs("-g", "daemon on;"),
							); err == nil {
								nginxErr = cmd.Run()
								if nginxErr != nil {
									slog.Warn("init nginx make resolver", "error", nginxErr)
								}
							}
						}
					}
				}
			} else {
				slog.Warn("docker daemon/event get dpanel container info", "error", err)
				// 如果在容器中找不到 dpanel 容器则后续不会挂载 dpanel 目录
				dockerEnv.DockerInfo.InDPanel = false
				result.ContainerInfo = container.InspectResponse{}
				result.Mount = types.VolumeItem{}
				result.DataMounts = nil
			}
		} else {
			result.RunIn = types2.DPanelRunInHost
			// 二进制运行时没有容器挂载信息，清理容器模式遗留的持久化数据。
			result.ContainerInfo = container.InspectResponse{}
			// 如果是二进制运行，则挂载数据存储目录
			// 如果在 windows 默认是远程 docker 那么需要转换一个安全路径
			// 否则保持原样就可以了
			result.Mount = types.VolumeItem{
				Host: storage.Local{}.GetStorageLocalPath(),
				Dest: "/dpanel",
				Type: types3.VolumeTypeBind,
			}
			if !dockerEnv.IsLocal() && runtime.GOOS == "windows" {
				if v, ok := function.WindowsPathToSlash(storage.Local{}.GetStorageLocalPath()); ok {
					result.Mount.Host = v
				}
			}
			result.DataMounts = []types.VolumeItem{result.Mount}
		}
		slog.Debug("docker daemon/event init dpanel info", "info", result)
		_ = logic.Setting{}.Save(&entity.Setting{
			GroupName: logic.SettingGroupSetting,
			Name:      logic.SettingGroupSettingDPanelInfo,
			Value: &accessor.SettingValueOption{
				DPanelInfo: &result,
			},
		})
		logic.Env{}.UpdateEnv(dockerEnv)
	}
}

func (self Docker) Message(e event.DockerMessagePayload) {
	var eventDockerClient *docker.Client
	if client, ok := notice.Monitor.Clients()[e.DockerEnvName]; ok {
		eventDockerClient = client
	} else if docker.Sdk != nil && docker.Sdk.Name == e.DockerEnvName {
		eventDockerClient = docker.Sdk
	}
	if eventDockerClient != nil {
		ctx := context.Background()
		eventDockerClient.ContainerRuntimeCollect(ctx, e.Message)
		if containerID := e.Message.Actor.Attributes["container"]; containerID != "" {
			if runtime, ok := eventDockerClient.ContainerRuntime(ctx, containerID); ok && runtime.ContainerName != "" {
				attributes := make(map[string]string, len(e.Message.Actor.Attributes)+1)
				for key, value := range e.Message.Actor.Attributes {
					attributes[key] = value
				}
				attributes["containerName"] = runtime.ContainerName
				e.Message.Actor.Attributes = attributes
			}
		}
	}

	containerName := e.Message.Actor.Attributes[define.DPanelLabelContainerName]
	if containerName == "" {
		containerName = e.Message.Actor.Attributes["name"]
	}
	if !function.InArray(dockerMessageFilterContainerList, containerName) {
		e.ID = uuid.NewString()
		e.Level = types2.LogLevelInfo
		action, _, _ := strings.Cut(string(e.Message.Action), ": ")
		if level, ok := dockerMessageLevelMap[dockerEvents.Action(action)]; ok {
			e.Level = level
		}
		dockerMessageMu.Lock()
		dockerMessages := make([]*event.DockerMessagePayload, 0, maxDockerMessageSize)
		if value, ok := storage.Cache.Get(storage.CacheKeyDockerEvents); ok {
			if cached, ok := value.([]*event.DockerMessagePayload); ok {
				dockerMessages = append(dockerMessages, cached...)
			}
		}
		dockerMessages = append(dockerMessages, &e)
		if len(dockerMessages) > maxDockerMessageSize {
			dockerMessages = dockerMessages[len(dockerMessages)-maxDockerMessageSize:]
		}
		storage.Cache.Set(storage.CacheKeyDockerEvents, dockerMessages, cache.DefaultExpiration)
		dockerMessageMu.Unlock()
	}

	msgType := string(e.Message.Type) + "/" + string(e.Message.Action)
	if function.InArray([]string{
		define.DockerMessageTypeContainerDestroy, define.DockerMessageTypeContainerCreate,
		define.DockerMessageTypeContainerDie, define.DockerMessageTypeContainerStart,
	}, msgType) {
		environment := []types.EnvItem{
			types.NewEnvItemFromKV("DP_DOCKER_ENV_NAME", e.DockerEnvName),
			types.NewEnvItemFromKV("DP_CONTAINER_NAME", e.Message.Actor.Attributes["name"]),
		}
		for _, job := range crontab.Client.GetJobs(fmt.Sprintf(logic.CronEventJobSearch, msgType)) {
			job.Run(crontab.WithEnvironment(environment))
		}
	}
}
