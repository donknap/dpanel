package stat

import (
	"context"
	"errors"
	"log/slog"
	"sort"

	apiTypes "github.com/docker/docker/api/types"
	"github.com/donknap/dpanel/common/service/docker"
	dockerTypes "github.com/donknap/dpanel/common/service/docker/types"
)

func (self Stat) ReadDockerStat(ctx context.Context, dockerSdk *docker.Client) <-chan DockerStatFrame {
	result := make(chan DockerStatFrame)
	go func() {
		defer close(result)

		source, err := dockerSdk.ContainerStats(ctx, dockerTypes.ContainerStatsOption{Stream: true})
		if err != nil {
			if ctx.Err() == nil && !errors.Is(err, context.Canceled) {
				slog.Warn("stream Docker stat", "dockerEnvName", dockerSdk.Name, "error", err)
			}
			return
		}
		for {
			select {
			case value, ok := <-source:
				if !ok {
					return
				}
				select {
				case result <- DockerStatFrame(value):
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return result
}

func (self Stat) DockerDiskUsage(ctx context.Context, dockerSdk *docker.Client) (apiTypes.DiskUsage, error) {
	usage, err := dockerSdk.Client.DiskUsage(ctx, apiTypes.DiskUsageOptions{
		Types: []apiTypes.DiskUsageObject{
			apiTypes.ContainerObject,
			apiTypes.ImageObject,
			apiTypes.VolumeObject,
			apiTypes.BuildCacheObject,
		},
	})
	if err != nil {
		return apiTypes.DiskUsage{}, err
	}
	for index := range usage.Containers {
		usage.Containers[index].Labels = make(map[string]string)
	}
	for index := range usage.Images {
		usage.Images[index].Labels = make(map[string]string)
	}
	for index := range usage.Volumes {
		usage.Volumes[index].Labels = make(map[string]string)
	}
	sort.Slice(usage.Images, func(i, j int) bool { return usage.Images[i].Size > usage.Images[j].Size })
	sort.Slice(usage.Containers, func(i, j int) bool {
		return usage.Containers[i].SizeRw+usage.Containers[i].SizeRootFs >
			usage.Containers[j].SizeRw+usage.Containers[j].SizeRootFs
	})
	sort.Slice(usage.Volumes, func(i, j int) bool {
		if usage.Volumes[i].UsageData != nil && usage.Volumes[j].UsageData != nil {
			return usage.Volumes[i].UsageData.Size > usage.Volumes[j].UsageData.Size
		}
		return false
	})
	return usage, nil
}
