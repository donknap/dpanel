package docker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	dockerRegistry "github.com/docker/docker/api/types/registry"
	"github.com/donknap/dpanel/common/function"
	dockerTypes "github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/types/define"
	registrySdk "github.com/we7coreteam/registry-go-sdk"
)

var registryManifestAccepts = []string{
	"application/vnd.oci.image.index.v1+json",
	"application/vnd.docker.distribution.manifest.list.v2+json",
	"application/vnd.oci.image.manifest.v1+json",
	"application/vnd.docker.distribution.manifest.v2+json",
	"application/vnd.docker.distribution.manifest.v1+prettyjws",
	"application/vnd.docker.distribution.manifest.v1+json",
}

func (self Client) ImageDigestInspect(ctx context.Context, imageID, imageName string, option dockerTypes.ImageDigestInspectOption) (result dockerTypes.ImageDigestInspectResult, err error) {
	result = dockerTypes.ImageDigestInspectResult{
		ImageID:      strings.TrimSpace(imageID),
		ImageName:    strings.TrimSpace(imageName),
		LocalDigests: make([]string, 0),
	}
	if result.ImageID == "" {
		return result, errors.New("image id is empty")
	}
	if result.ImageName == "" {
		return result, errors.New("image name is empty")
	}

	inspectContext, inspectCancel := context.WithTimeout(ctx, define.DockerConnectServerTimeout)
	imageInfo, err := self.Client.ImageInspect(inspectContext, result.ImageID)
	inspectCancel()
	if err != nil {
		return result, fmt.Errorf("inspect local image %s: %w", result.ImageID, err)
	}
	result.ImageID = imageInfo.ID
	result.LocalDigests = append(result.LocalDigests, imageInfo.RepoDigests...)
	if len(result.LocalDigests) == 0 {
		return result, nil
	}

	imageNameDetail := function.ImageTag(result.ImageName)
	if imageNameDetail.BaseName == "" || imageNameDetail.Version == "" || imageNameDetail.Registry == "" {
		return result, fmt.Errorf("invalid image name: %s", result.ImageName)
	}
	registryConfig := option.Registry
	if registryConfig.Address == "" {
		registryConfig.Address = imageNameDetail.Registry
	}
	proxyAddresses, sourceAddress := registryConfig.APIAddresses()
	var lastErr error
	username := ""
	password := ""
	if option.Registry.Auth != "" {
		authConfig, err := dockerRegistry.DecodeAuthConfig(option.Registry.Auth)
		if err != nil {
			return result, fmt.Errorf("decode registry auth: %w", err)
		}
		username = authConfig.Username
		password = authConfig.Password
	}
	inspect := func(address, username, password string, attemptContext context.Context) (bool, string, error) {
		registryClient := registrySdk.New(registrySdk.WithServer(
			address,
			username,
			password,
		)).Client(dockerTypes.HTTPContextInterceptor{Context: attemptContext})
		request, err := http.NewRequest(
			http.MethodHead,
			fmt.Sprintf("%s/v2/%s/manifests/%s", strings.TrimSuffix(registryClient.Address(), "/"), imageNameDetail.BaseName, imageNameDetail.Version),
			nil,
		)
		if err != nil {
			return false, "", err
		}
		for _, mediaType := range registryManifestAccepts {
			request.Header.Add("Accept", mediaType)
		}
		response, err := registryClient.Do(request)
		if err != nil {
			if strings.Contains(err.Error(), "http status code: NOT_FOUND,") {
				return false, "", nil
			}
			return false, "", err
		}
		defer response.Body.Close()
		return true, response.Header.Get("Docker-Content-Digest"), nil
	}
	// 更新检测与拉取保持同一顺序：代理地址匿名优先，最后才使用带权限的源仓库。
	// 完全未匹配到仓库配置时，Registry 只包含镜像原地址，因此会直接检测原仓库。
	for _, address := range append(append([]string(nil), proxyAddresses...), sourceAddress) {
		if address == "" {
			continue
		}
		attemptUsername := ""
		attemptPassword := ""
		if address == sourceAddress {
			attemptUsername = username
			attemptPassword = password
		}
		attemptContext, attemptCancel := context.WithTimeout(ctx, define.DockerConnectServerTimeout)
		ok, remoteDigest, err := inspect(address, attemptUsername, attemptPassword, attemptContext)
		attemptCancel()
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		// 后续地址覆盖前序代理结果，确保最终由排在最后的源仓库决定失败或不可用。
		// 已成功请求但不存在 manifest 属于不可用状态，而非检查失败。
		lastErr = err
		if err != nil {
			slog.Debug("inspect image digest from registry", "server", address, "error", err)
			continue
		}
		if ok && remoteDigest != "" {
			result.RemoteDigest = remoteDigest
			return result, nil
		}
	}
	if lastErr != nil {
		return result, fmt.Errorf("get remote manifest %s:%s: %w", imageNameDetail.BaseName, imageNameDetail.Version, lastErr)
	}
	return result, nil
}
