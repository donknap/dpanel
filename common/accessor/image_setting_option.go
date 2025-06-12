package accessor

import "github.com/donknap/dpanel/common/service/docker"

type ImageSettingOption struct {
	BuildGit            string                `json:"buildGit"`
	BuildDockerfile     string                `json:"buildDockerfile,omitempty"`
	BuildRoot           string                `json:"buildRoot,omitempty"`
	BuildDockerfileName string                `json:"buildDockerfileName,omitempty"`
	BuildArgs           []docker.EnvItem      `json:"buildArgs,omitempty"`
	Platform            *docker.ImagePlatform `json:"platform,omitempty"`
	Registry            string                `json:"registry"`
}
