package logic

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/patrickmn/go-cache"
	registrySdk "github.com/we7coreteam/registry-go-sdk"
)

const containerUpgradeCacheDuration = 10 * time.Minute

type ContainerUpgradeResult struct {
	DockerEnvName string
	CheckedAt     string
	ContainerID   string
	ContainerName string
	Error         error
	ImageID       string
	ImageName     string
	LocalDigest   []string
	RemoteDigest  string
	Status        string
	checkedTime   time.Time
}

type ContainerUpgradeProgress struct {
	Steps   []string `json:"steps"`
	Current int      `json:"current"`
	Total   int      `json:"total"`
}

type ContainerUpgrade struct{}

type registryRequestContext struct {
	context.Context
}

func (self registryRequestContext) Intercept(request *http.Request) error {
	*request = *request.WithContext(self.Context)
	return nil
}

func (ContainerUpgrade) Check(dockerSdk *docker.Client, containerInfo *container.InspectResponse, force bool) (result ContainerUpgradeResult, err error) {
	result = ContainerUpgradeResult{
		DockerEnvName: dockerSdk.Name,
		ContainerID:   containerInfo.ID,
		ContainerName: containerInfo.Name,
		ImageID:       containerInfo.Image,
		LocalDigest:   make([]string, 0),
	}
	if containerInfo.Config != nil {
		result.ImageName = containerInfo.Config.Image
	}
	defer func() {
		if result.checkedTime.IsZero() {
			result.checkedTime = time.Now()
			result.CheckedAt = result.checkedTime.Format(define.DateShowYmdHis)
		}
		storage.Cache.Set(
			fmt.Sprintf(storage.CacheKeyContainerUpgrade, result.DockerEnvName, result.ContainerID),
			result,
			cache.NoExpiration,
		)
	}()

	checkContext, cancel := context.WithTimeout(dockerSdk.Ctx, define.DockerConnectServerTimeout)
	defer cancel()

	if !force {
		if value, exists := storage.Cache.Get(fmt.Sprintf(storage.CacheKeyContainerUpgrade, result.DockerEnvName, result.ContainerID)); exists {
			if cached, ok := value.(ContainerUpgradeResult); ok &&
				time.Since(cached.checkedTime) < containerUpgradeCacheDuration {
				cached.ContainerName = result.ContainerName
				return cached, cached.Error
			}
		}
	}

	imageInfo, err := dockerSdk.Client.ImageInspect(checkContext, result.ImageID)
	if err != nil {
		result.Error = fmt.Errorf("inspect local image %s: %w", result.ImageID, err)
		result.Status = define.ContainerUpgradeStatusFailed
		return result, result.Error
	}
	result.LocalDigest = append(result.LocalDigest, imageInfo.RepoDigests...)
	if len(result.LocalDigest) == 0 {
		result.Status = define.ContainerUpgradeStatusUnavailable
		return result, nil
	}

	imageNameDetail := function.ImageTag(result.ImageName)
	if imageNameDetail.BaseName == "" || imageNameDetail.Version == "" || imageNameDetail.Registry == "" {
		result.Error = fmt.Errorf("invalid image name: %s", result.ImageName)
		result.Status = define.ContainerUpgradeStatusFailed
		return result, result.Error
	}
	registryConfig := (Image{}).GetRegistryConfig(imageNameDetail.Registry)
	registryCredential := registryConfig.Credential()
	registryOptions := make([]registrySdk.Option, 0, len(registryConfig.Address))
	for _, address := range registryConfig.Address {
		registryOptions = append(registryOptions, registrySdk.WithServer(address, registryCredential.AccessKey, registryCredential.AccessSecret))
	}
	ok, manifest, err := registrySdk.New(registryOptions...).Client(registryRequestContext{Context: checkContext}).ManifestExist(imageNameDetail.BaseName, imageNameDetail.Version)
	if err != nil {
		result.Error = fmt.Errorf("get remote manifest %s:%s: %w", imageNameDetail.BaseName, imageNameDetail.Version, err)
		result.Status = define.ContainerUpgradeStatusFailed
		return result, result.Error
	}
	if !ok || manifest == nil {
		result.Error = fmt.Errorf("remote manifest not found: %s:%s", imageNameDetail.BaseName, imageNameDetail.Version)
		result.Status = define.ContainerUpgradeStatusUnavailable
		return result, result.Error
	}
	result.RemoteDigest = manifest.Digest.String()
	result.Status = define.ContainerUpgradeStatusLatest
	if !function.InArrayWalk(result.LocalDigest, func(localDigest string) bool {
		return strings.HasSuffix(localDigest, result.RemoteDigest)
	}) {
		result.Status = define.ContainerUpgradeStatusUpgrade
	}
	return result, nil
}
