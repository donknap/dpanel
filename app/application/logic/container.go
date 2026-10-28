package logic

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/agent/factor"
	"github.com/donknap/dpanel/common/service/docker"
	dockerTypes "github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/patrickmn/go-cache"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

type Container struct {
}

type ContainerRuntimeStatus struct {
	Unhealthy bool
	State     container.ContainerState
	Message   string
}

type ContainerRuntimeItem struct {
	Summary container.Summary
	Inspect *container.InspectResponse
}

type ContainerUpgradeCheckResult struct {
	CheckedAt string
	Error     string
	Result    dockerTypes.ImageDigestInspectResult
	Status    string
}

func (self Container) IsUpgradeIgnore(containerName string) bool {
	containerName = strings.TrimPrefix(strings.TrimSpace(containerName), "/")
	return containerName == strings.TrimPrefix(strings.TrimSpace(facade.GetConfig().GetString("app.name")), "/") || containerName == factor.MonitorName
}

func (self Container) CheckUpgrade(dockerSdk *docker.Client, containerInfo container.InspectResponse, force bool) ContainerUpgradeCheckResult {
	cacheKey := fmt.Sprintf(storage.CacheKeyContainerUpgradeCheck, dockerSdk.Name, containerInfo.ID)
	refreshCache := func() (cached ContainerUpgradeCheckResult) {
		imageName := ""
		var localDigestList []string
		if containerInfo.Config != nil {
			imageName = containerInfo.Config.Image
			localDigestList = strings.Split(containerInfo.Config.Labels[define.DPanelLabelImageDigest], ",")
		}
		cached = ContainerUpgradeCheckResult{
			CheckedAt: time.Now().Format(define.DateShowYmdHis),
			Result: dockerTypes.ImageDigestInspectResult{
				ImageID:      containerInfo.Image,
				ImageName:    imageName,
				LocalDigests: make([]string, 0),
			},
			Status: define.ContainerUpgradeStatusLatest,
		}
		defer func() {
			storage.Cache.Set(cacheKey, cached, cache.NoExpiration)
		}()
		if strings.Contains(imageName, "@") || strings.HasPrefix(imageName, "sha256:") || function.IsDockerObjectID(imageName) {
			cached.Error = "container uses a fixed image ID or digest; cannot check for updates"
			cached.Status = define.ContainerUpgradeStatusUnavailable
			return cached
		}
		inspectContext, inspectCancel := context.WithTimeout(dockerSdk.Ctx, define.DockerConnectServerTimeout)
		imageInfo, err := dockerSdk.Client.ImageInspect(inspectContext, imageName)
		inspectCancel()
		if err != nil && !errdefs.IsNotFound(err) {
			cached.Error = fmt.Sprintf("inspect local image %s: %v", imageName, err)
			cached.Status = define.ContainerUpgradeStatusFailed
			return cached
		}
		// 同名标签已指向另一镜像时，容器仍使用旧镜像，无需查询远端摘要。
		if err == nil && imageInfo.ID != containerInfo.Image {
			cached.Status = define.ContainerUpgradeStatusUpgrade
			return cached
		}
		imageNameDetail := function.ImageTag(imageName)
		registryConfig := Image{}.GetRegistryConfig(imageNameDetail.Registry)
		result, err := dockerSdk.ImageDigestInspect(dockerSdk.Ctx, containerInfo.Image, imageName, dockerTypes.ImageDigestInspectOption{
			Registry:        *registryConfig,
			LocalDigestList: localDigestList,
		})
		cached.Result = result
		cached.Error = result.Error
		if err != nil {
			cached.Error = err.Error()
			cached.Status = define.ContainerUpgradeStatusFailed
		} else if !result.IsAvailable() {
			cached.Status = define.ContainerUpgradeStatusUnavailable
		} else if result.IsDifferent() {
			cached.Status = define.ContainerUpgradeStatusUpgrade
		}
		return cached
	}

	if force {
		return refreshCache()
	}
	cached, _ := storage.LoadCacheOrStore(cacheKey, refreshCache)
	return cached
}

func (self Container) RuntimeStatus(item ContainerRuntimeItem) ContainerRuntimeStatus {
	result := ContainerRuntimeStatus{}
	if strings.Contains(item.Summary.Status, "unhealthy") || strings.Contains(item.Summary.Status, "Restarting") {
		result.Unhealthy = true
	}
	if item.Inspect == nil {
		return result
	}
	if item.Inspect.State != nil && item.Inspect.State.Restarting && item.Inspect.RestartCount > 0 {
		result.Unhealthy = true
		result.State = container.ContainerState(container.Unhealthy)
		result.Message = "Frequent restarts"
		return result
	}
	if item.Inspect.Config == nil || item.Inspect.Config.Healthcheck != nil {
		return result
	}
	runtime, ok := docker.Sdk.ContainerRuntime(docker.Sdk.Ctx, item.Inspect.ID)
	if !ok {
		return result
	}

	since := time.Now().Add(-time.Minute)
	actionCount := 0
	for _, item := range runtime.History {
		if item.Time.Before(since) {
			continue
		}
		if item.Action == "start" || item.Action == "restart" {
			actionCount += 1
		}
	}
	if actionCount >= 3 {
		result.Unhealthy = true
		result.State = container.ContainerState(container.Unhealthy)
		result.Message = "Frequent restarts"
	}
	return result
}
