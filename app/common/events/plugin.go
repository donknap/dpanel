package events

import (
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/plugin"
	"github.com/donknap/dpanel/common/types/event"
)

type Plugin struct {
}

func (self Plugin) DestroyExplorer(e event.DockerDaemonPayload) {
	if dockerEnv, err := (logic.Env{}).GetEnvByName(e.DockerEnvName); err == nil {
		if dockerSdk, err := docker.NewClientWithDockerEnv(dockerEnv); err == nil {
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
}
