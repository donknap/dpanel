package events

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/function"
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
				var removeErr error
				for _, containerInfo := range list {
					err = dockerSdk.Client.ContainerStop(dockerSdk.Ctx, containerInfo.ID, container.StopOptions{})
					if err != nil {
						errors.Join(removeErr, err)
					}
					err = dockerSdk.Client.ContainerRemove(dockerSdk.Ctx, containerInfo.ID, container.RemoveOptions{
						Force: true,
					})
					if err != nil {
						errors.Join(removeErr, err)
					}
					err = dockerSdk.ImageRemoveAll(dockerSdk.Ctx, containerInfo.Image)
					if err != nil {
						errors.Join(removeErr, err)
					}
				}
				slog.Debug("plugin destroy explorer", "name", function.PluckArrayWalk(list, func(item container.Summary) ([]string, bool) {
					return item.Names, true
				}), "error", removeErr)
			}
			return
		}
	}
}
