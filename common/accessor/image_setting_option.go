package accessor

import (
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker/types"
)

type ImageSettingOption struct {
	ImageId                string               `json:"imageId"`
	Tag                    string               `json:"tag,omitempty"` // Deprecated: instead Tags
	Tags                   []*function.Tag      `json:"tags,omitempty" binding:"required"`
	Registry               string               `json:"registry,omitempty"`
	BuildType              string               `json:"buildType,omitempty"`
	BuildDockerfileContent string               `json:"buildDockerfileContent" binding:"omitempty"`
	BuildDockerfileName    string               `json:"buildDockerfileName"`
	BuildDockerfileRoot    string               `json:"buildDockerfileRoot"`
	BuildGit               string               `json:"buildGit"`
	BuildZip               string               `json:"buildZip"`
	BuildArgs              []types.EnvItem      `json:"buildArgs,omitempty"`
	Platform               string               `json:"platform,omitempty"` // Deprecated: instead PlatformArch
	PlatformArch           *types.ImagePlatform `json:"platformArch,omitempty"`
	BuildDockerfile        string               `json:"buildDockerfile,omitempty,deprecated"` // Deprecated: instead BuildDockerfileContent
	BuildRoot              string               `json:"buildRoot,omitempty,deprecated"`       // Deprecated: instead BuildDockerfileRoot
}
