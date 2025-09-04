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
	Id                     int32 `json:"id"`
	MessageId              string
	Registry               string               `json:"registry"`
	Tag                    string               `json:"tag" binding:"required"`
	Title                  string               `json:"title"`
	BuildType              string               `json:"buildType" binding:"required"`
	BuildDockerfileContent string               `json:"buildDockerfileContent" binding:"omitempty"`
	BuildDockerfileName    string               `json:"buildDockerfileName"`
	BuildDockerfileRoot    string               `json:"buildDockerfileRoot"`
	BuildGit               string               `json:"buildGit" binding:"omitempty"`
	BuildZip               string               `json:"buildZip" binding:"omitempty"`
	BuildArgs              []docker.EnvItem     `json:"buildArgs"`
	Platform               docker.ImagePlatform `json:"platform"`
	EnablePush             bool                 `json:"enablePush,omitempty"`
}

type ImageRemoteOption struct {
	Auth     string
	Type     string
	Tag      string
	Platform string
	Proxy    string
}

type NoticeOption struct {
	Message string
}

type DockerTask struct {
	sdk *docker.Builder
}
