package main

import (
	checkhandler "github.com/donknap/dpanel/app/agent/handler/check"
	fshandler "github.com/donknap/dpanel/app/agent/handler/fs"
	importhandler "github.com/donknap/dpanel/app/agent/handler/importer"
	stathandler "github.com/donknap/dpanel/app/agent/handler/stat"
	usagehandler "github.com/donknap/dpanel/app/agent/handler/usage"
	versionhandler "github.com/donknap/dpanel/app/agent/handler/version"
	"github.com/donknap/dpanel/app/agent/internal"
)

type Provider struct{}

func (*Provider) Register(handlers internal.Handlers) {
	handlers["fs"] = fshandler.New()
	handlers["check"] = checkhandler.New()
	handlers["import"] = importhandler.New()
	handlers["stat"] = stathandler.New()
	handlers["usage"] = usagehandler.New()
	handlers["version"] = versionhandler.New(version)
}
