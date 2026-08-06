package docker

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/donknap/dpanel/common/function"
	dockerTypes "github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/types/define"
	registrySdk "github.com/we7coreteam/registry-go-sdk"
)

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

	inspectContext, cancel := context.WithTimeout(ctx, define.DockerConnectServerTimeout)
	defer cancel()
	imageInfo, err := self.Client.ImageInspect(inspectContext, result.ImageID)
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
	registryAddresses := option.RegistryAddresses
	if len(registryAddresses) == 0 {
		registryAddresses = []string{imageNameDetail.Registry}
	}
	registryOptions := make([]registrySdk.Option, 0, len(registryAddresses)+1)
	for _, address := range registryAddresses {
		registryOptions = append(registryOptions, registrySdk.WithServer(
			address,
			option.RegistryCredential.AccessKey,
			option.RegistryCredential.AccessSecret,
		))
	}
	registryOptions = append(registryOptions, registrySdk.WithRepository(imageNameDetail.BaseName, imageNameDetail.Version))
	ok, manifest, err := registrySdk.New(registryOptions...).Client(dockerTypes.HTTPContextInterceptor{Context: inspectContext}).ManifestExist(
		imageNameDetail.BaseName,
		imageNameDetail.Version,
	)
	if err != nil {
		return result, fmt.Errorf("get remote manifest %s:%s: %w", imageNameDetail.BaseName, imageNameDetail.Version, err)
	}
	if !ok || manifest == nil {
		return result, nil
	}
	result.RemoteDigest = manifest.Digest.String()
	return result, nil
}
