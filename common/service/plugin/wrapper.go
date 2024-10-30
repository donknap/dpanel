package plugin

import (
	"github.com/donknap/dpanel/common/service/docker"
)

type Wrapper struct {
}

type pluginItem struct {
	Name        string `json:"name"`
	IsCreated   bool   `json:"isCreated"`
	ContainerId string `json:"containerId"`
}

func (self Wrapper) GetPluginList() map[string]pluginItem {
	pluginList := []string{
		PluginExplorer, PluginBackup,
	}
	r := make(map[string]pluginItem)
	for _, name := range pluginList {
		containerRow, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, "dpanel-plugin-"+name)
		if err == nil {
			r[name] = pluginItem{
				Name:        name,
				IsCreated:   true,
				ContainerId: containerRow.ID,
			}
		} else {
			r[name] = pluginItem{
				Name:      name,
				IsCreated: false,
			}
		}
	}
	return r
}
