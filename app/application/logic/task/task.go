package task

import (
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/service/docker"
)

type CreateContainerOption struct {
	SiteTitle   string `json:"siteTitle"`
	SiteName    string `json:"siteName" binding:"required"`
	ImageName   string `json:"imageName" binding:"required"`
	ContainerId string `json:"id"`
	// ImageAutoCommitMerge 是本次容器重建的镜像层合并开关，独立于 BuildParams，避免持久化后在后续重建中重复执行。
	ImageAutoCommitMerge bool                    `json:"imageAutoCommitMerge"`
	BuildParams          *accessor.SiteEnvOption `json:"-"`
}

type ImageRemoteOption struct {
	Auth     string
	Type     string
	Tag      string
	Platform string
	Proxy    string
}

type Docker struct {
	sdk *docker.Client
}
