package main

import (
	fshandler "github.com/donknap/dpanel/app/agent/handler/fs"
	porthandler "github.com/donknap/dpanel/app/agent/handler/port"
	stathandler "github.com/donknap/dpanel/app/agent/handler/stat"
	usagehandler "github.com/donknap/dpanel/app/agent/handler/usage"
	versionhandler "github.com/donknap/dpanel/app/agent/handler/version"
	"github.com/donknap/dpanel/app/agent/internal"
)

type Provider struct{}

func (*Provider) Register(handlers internal.Handlers) {
	handlers["fs"] = fshandler.New()
	handlers["port"] = porthandler.New()
	handlers["stat"] = stathandler.New()
	handlers["usage"] = usagehandler.New()
	handlers["version"] = versionhandler.New(version)
}
