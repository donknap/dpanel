package logic

import (
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/service/docker"
)

type CreateContainerOption struct {
	SiteId      int32  `json:"id"` // 站点id
	SiteTitle   string `json:"siteTitle" binding:"required"`
	SiteName    string `json:"siteName" binding:"required"`
	ContainerId string `json:"containerId"`
	BuildParams *accessor.SiteEnvOption
}

type BuildImageOption struct {
	ZipPath           string // 构建包
	DockerFileContent []byte // 自定义Dockerfile
	DockerFileInPath  string // Dockerfile 所在路径
	GitUrl            string
	Tag               string // 镜像Tag
	ImageId           int32
	Context           string // Dockerfile 所在的目录
	Platform          *Platform
}

type Platform struct {
	Type string
	Arch string
}

type ImageRemoteOption struct {
	Auth     string
	Type     string
	Tag      string
	Platform string
}

type NoticeOption struct {
	Message string
}

type DockerTask struct {
	sdk *docker.Builder
}
