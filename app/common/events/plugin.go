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
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/donknap/dpanel/common/types/event"
)

type Plugin struct {
}

func (self Plugin) Destroy(e event.DockerDaemonPayload) {
	storage.Cache.Delete(fmt.Sprintf(storage.CacheKeyExplorerAfs, e.DockerEnvName, plugin.ExplorerName))
	if dockerEnv, err := (logic.Env{}).GetEnvByName(e.DockerEnvName); err == nil {
		if dockerSdk, err := docker.NewClientWithDockerEnv(dockerEnv); err == nil {
			defer dockerSdk.Close()
			filter := filters.NewArgs()
			filter.Add(docker.ContainerFilterLabel, fmt.Sprintf("%s=true", define.DPanelLabelContainerAutoRemove))
			if list, err := dockerSdk.ContainerSearchList(dockerSdk.Ctx, container.ListOptions{
				All:     true,
				Filters: filter,
			}); err == nil {
				var removeErr error
				images := make(map[string]struct{})
				for _, containerInfo := range list {
					images[containerInfo.Image] = struct{}{}
					err = dockerSdk.Client.ContainerStop(dockerSdk.Ctx, containerInfo.ID, container.StopOptions{})
					if err != nil {
						removeErr = errors.Join(removeErr, err)
					}
					err = dockerSdk.Client.ContainerRemove(dockerSdk.Ctx, containerInfo.ID, container.RemoveOptions{
						Force: true,
					})
					if err != nil {
						removeErr = errors.Join(removeErr, err)
					}
				}
				for imageName := range images {
					if err = dockerSdk.ImageRemove(dockerSdk.Ctx, filters.NewArgs(
						filters.Arg(docker.ImageFilterReference, imageName),
					)); err != nil {
						removeErr = errors.Join(removeErr, err)
					}
				}
				slog.Debug("plugin destroy", "name", function.PluckArrayWalk(list, func(item container.Summary) ([]string, bool) {
					return item.Names, true
				}), "error", removeErr)
			}
			return
		}
	}
}
