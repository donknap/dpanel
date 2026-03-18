package events

import (
	"fmt"
	"log/slog"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/plugin"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/donknap/dpanel/common/types/event"
)

type Plugin struct {
}

func (self Plugin) DestroyExplorer(e event.DockerDaemonPayload) {
	if dockerEnv, err := (logic.Env{}).GetEnvByName(e.DockerEnvName); err == nil {
		if dockerSdk, err := docker.NewClientWithDockerEnv(dockerEnv); err == nil {
			defer dockerSdk.Close()
			filter := filters.NewArgs()
			filter.Add("label", fmt.Sprintf("%s=%s", define.DPanelLabelContainerName, plugin.ExplorerName))
			if list, err := dockerSdk.Client.ContainerList(dockerSdk.Ctx, container.ListOptions{
				All:     true,
				Filters: filter,
			}); err == nil {
				for _, containerInfo := range list {
					slog.Debug("plugin destroy explorer", "name", containerInfo.Names)
					_ = dockerSdk.Client.ContainerStop(dockerSdk.Ctx, containerInfo.ID, container.StopOptions{})
					_ = dockerSdk.Client.ContainerRemove(dockerSdk.Ctx, containerInfo.ID, container.RemoveOptions{
						Force: true,
					})
				}
			}
			return
		}
	}
}
