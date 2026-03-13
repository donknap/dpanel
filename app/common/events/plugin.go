package events

import (
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/plugin"
	"github.com/donknap/dpanel/common/types/event"
)

type Plugin struct {
}

func (self Plugin) DestroyExplorer(e event.DockerDaemonPayload) {
	if dockerSdk, err := docker.NewClientWithDockerEnv(e.DockerEnv); err == nil {
		defer dockerSdk.Close()
		explorer, err := plugin.NewPlugin(dockerSdk, plugin.ExplorerName, plugin.CreateOption{})
		if err != nil {
			return
		}
		err = explorer.Close()
		if err != nil {
			return
		}
		return
	}
}
