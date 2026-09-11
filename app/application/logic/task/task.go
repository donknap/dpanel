package task

import (
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/service/docker"
)

type CreateContainerOption struct {
	SiteTitle   string                  `json:"siteTitle"`
	SiteName    string                  `json:"siteName" binding:"required"`
	ImageName   string                  `json:"imageName" binding:"required"`
	ContainerId string                  `json:"id"`
	BuildParams *accessor.SiteEnvOption `json:"-"`
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
