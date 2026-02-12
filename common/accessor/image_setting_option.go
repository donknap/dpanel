package accessor

import (
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker/types"
)

type ImageSettingOption struct {
	ImageId                string            `json:"imageId"`
	Tag                    string            `json:"tag,omitempty"` // Deprecated: instead Tags
	Tags                   []ImageSettingTag `json:"tags,omitempty" binding:"required"`
	Registry               string            `json:"registry,omitempty"`
	BuildEngine            string            `json:"buildEngine"`
	BuildType              string            `json:"buildType,omitempty"`
	BuildDockerfileContent string            `json:"buildDockerfileContent" binding:"omitempty"`
	BuildDockerfileName    string            `json:"buildDockerfileName"`
	BuildDockerfileRoot    string            `json:"buildDockerfileRoot"`
	BuildGit               string            `json:"buildGit"`
	BuildPath              string            `json:"buildPath"`
	BuildZip               string            `json:"buildZip"`
	BuildArgs              []types.EnvItem   `json:"buildArgs,omitempty"`
	BuildSecret            []types.EnvItem   `json:"buildSecret,omitempty"`
	BuildPlatformType      []string          `json:"buildPlatformType,omitempty"`
	BuildEnablePush        bool              `json:"buildEnablePush,omitempty"`
	BuildCacheType         string            `json:"buildCacheType"`
	UseTime                float64           `json:"useTime"`
	BuildDockerfile        string            `json:"buildDockerfile,omitempty,deprecated"` // Deprecated: instead BuildDockerfileContent
	BuildRoot              string            `json:"buildRoot,omitempty,deprecated"`       // Deprecated: instead BuildDockerfileRoot
}

type ImageSettingTag struct {
	*function.Tag
	Target string `json:"target"`
	Enable bool   `json:"enable"`
}
