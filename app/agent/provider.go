package main

import (
	fshandler "github.com/donknap/dpanel/app/agent/handler/fs"
	importhandler "github.com/donknap/dpanel/app/agent/handler/importer"
	porthandler "github.com/donknap/dpanel/app/agent/handler/port"
	stathandler "github.com/donknap/dpanel/app/agent/handler/stat"
	usagehandler "github.com/donknap/dpanel/app/agent/handler/usage"
	versionhandler "github.com/donknap/dpanel/app/agent/handler/version"
	"github.com/donknap/dpanel/app/agent/internal"
)

type Provider struct{}

func (*Provider) Register(handlers internal.Handlers) {
	handlers["fs"] = fshandler.New()
	handlers["import"] = importhandler.New()
	handlers["check-port"] = porthandler.New()
	handlers["stat"] = stathandler.New()
	handlers["usage"] = usagehandler.New()
	handlers["version"] = versionhandler.New(version)
}
