package events

import (
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"sync"

	types3 "github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/docker/api/types/container"
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
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/service/ws"
	types2 "github.com/donknap/dpanel/common/types"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/donknap/dpanel/common/types/event"
	"github.com/gin-gonic/gin"
	"github.com/patrickmn/go-cache"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

const maxEventCacheSize = 100

var (
	eventCache = make([]*event.DockerMessagePayload, 0, maxEventCacheSize)
	mu         = sync.Mutex{}
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

	// 如果有错误记录缓存返回,并删除缓存信息，默认为的状态为 false
	if !e.Status.Available {
		slog.Debug("docker daemon/event delete cache", "name", e.Status)
		storage.Cache.Delete(dockerStatusCacheKey)
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
			if v, err := docker.NewClientWithDockerEnv(dockerEnv, docker.WithSockProxy()); err == nil {
				docker.Sdk = v
				slog.Debug("docker daemon/event update docker.Sdk")
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
	if result.Proxy != "" {
		_ = os.Setenv("HTTP_PROXY", result.Proxy)
		_ = os.Setenv("HTTPS_PROXY", result.Proxy)
		slog.Info("init dpanel proxy", "url", result.Proxy)
	}

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
				if v, _, ok := function.PluckArrayItemWalk(info.Mounts, func(item container.MountPoint) bool {
					return item.Destination == "/dpanel"
				}); ok {
					result.Mount = types.VolumeItem{
						Host: v.Source,
						Dest: v.Destination,
						Type: string(v.Type),
					}
					if v.Type == types3.VolumeTypeVolume {
						result.Mount.Host = v.Name
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
			}
		} else {
			result.RunIn = types2.DPanelRunInHost
			// 如果是二进制运行，则挂载数据存储目录
			// 如果在 windows 默认是远程 docker 那么需要转换一个安全路径
			// 否则保持原样就可以了
			result.Mount = types.VolumeItem{
				Host: storage.Local{}.GetStorageLocalPath(),
				Dest: "/dpanel",
				Type: types3.VolumeTypeBind,
			}
			if !dockerEnv.IsLocal() && runtime.GOOS == "windows" {
				if v, ok := function.PathConvertWinPath2Unix(storage.Local{}.GetStorageLocalPath()); ok {
					result.Mount.Host = v
				}
			}
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
	mu.Lock()
	eventCache = append(eventCache, &e)

	if len(eventCache) > maxEventCacheSize {
		eventCache = eventCache[len(eventCache)-maxEventCacheSize:]
	}

	storage.Cache.Set(storage.CacheKeyDockerEvents, eventCache, cache.DefaultExpiration)
	mu.Unlock()

	msgType := string(e.Message.Type) + "/" + string(e.Message.Action)
	if function.InArray([]string{
		define.DockerMessageTypeContainerDestroy, define.DockerMessageTypeContainerCreate,
		define.DockerMessageTypeContainerDie, define.DockerMessageTypeContainerStart,
	}, msgType) {
		crontab.Client.RunByEvent(msgType, []types.EnvItem{
			types.NewEnvItemFromKV("DP_DOCKER_ENV_NAME", e.DockerEnvName),
			types.NewEnvItemFromKV("DP_CONTAINER_NAME", e.Message.Actor.Attributes["name"]),
		})
	}
}
