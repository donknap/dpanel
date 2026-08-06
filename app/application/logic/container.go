package logic

import (
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	dockerTypes "github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
)

const containerUpgradeCacheDuration = 10 * time.Minute

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

func (self Container) CheckUpgrade(dockerSdk *docker.Client, containerInfo container.InspectResponse, force bool) ContainerUpgradeCheckResult {
	cacheKey := fmt.Sprintf(storage.CacheKeyContainerUpgrade, dockerSdk.Name, containerInfo.ID)
	refreshCache := func() ContainerUpgradeCheckResult {
		imageName := ""
		if containerInfo.Config != nil {
			imageName = containerInfo.Config.Image
		}
		imageNameDetail := function.ImageTag(imageName)
		registryConfig := Image{}.GetRegistryConfig(imageNameDetail.Registry)
		result, checkErr := dockerSdk.ImageDigestInspect(dockerSdk.Ctx, containerInfo.Image, imageName, dockerTypes.ImageDigestInspectOption{
			RegistryAddresses:  registryConfig.Address,
			RegistryCredential: registryConfig.Credential(),
		})
		errorMessage := ""
		status := define.ContainerUpgradeStatusLatest
		if checkErr != nil {
			errorMessage = checkErr.Error()
			status = define.ContainerUpgradeStatusFailed
		} else if !result.IsAvailable() {
			status = define.ContainerUpgradeStatusUnavailable
		} else if result.IsDifferent() {
			status = define.ContainerUpgradeStatusUpgrade
		}
		cached := ContainerUpgradeCheckResult{
			CheckedAt: time.Now().Format(define.DateShowYmdHis),
			Error:     errorMessage,
			Result:    result,
			Status:    status,
		}
		storage.Cache.Set(cacheKey, cached, containerUpgradeCacheDuration)
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
