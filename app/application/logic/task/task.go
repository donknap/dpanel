package task

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
