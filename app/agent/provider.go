package main

import (
	fshandler "github.com/donknap/dpanel/app/agent/handler/fs"
	usagehandler "github.com/donknap/dpanel/app/agent/handler/usage"
	versionhandler "github.com/donknap/dpanel/app/agent/handler/version"
	"github.com/donknap/dpanel/app/agent/internal"
)

type Provider struct{}

func (*Provider) Register(handlers internal.Handlers) {
	handlers["fs"] = fshandler.New()
	handlers["usage"] = usagehandler.New()
	handlers["version"] = versionhandler.New(version)
}
