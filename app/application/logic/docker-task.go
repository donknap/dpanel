package logic

import (
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/service/docker"
)

type CreateMessage struct {
	SiteName  string // 站点标识
	SiteId    int32  // 站点id
	RunParams *accessor.SiteEnvOption
}

type BuildImageMessage struct {
	ZipPath           string // 构建包
	DockerFileContent []byte // 自定义Dockerfile
	DockerFileInPath  string // Dockerfile 所在路径
	GitUrl            string
	Tag               string // 镜像Tag
	ImageId           int32
	Context           string // Dockerfile 所在的目录
}

type ImageRemoteMessage struct {
	Auth string
	Type string
	Tag  string
}

type NoticeMessage struct {
	Message string
}

type DockerTask struct {
	sdk *docker.Builder
}
