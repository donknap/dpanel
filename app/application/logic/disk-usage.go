package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/plugin"
)

type DiskUsage struct{}

type diskUsageAgentData struct {
	Used      uint64 `json:"used"`
	Available uint64 `json:"available"`
	Total     uint64 `json:"total"`
}

type diskUsageAgentMessage struct {
	Data  *diskUsageAgentData `json:"data"`
	Error string              `json:"error"`
	Code  int                 `json:"code"`
}

func (DiskUsage) Get(ctx context.Context, dockerSdk *docker.Client) (accessor.DiskUsage, error) {
	result := accessor.DiskUsage{DockerEnvName: dockerSdk.Name}
	type dockerUsageResult struct {
		usage types.DiskUsage
		err   error
	}
	type hostUsageResult struct {
		usage diskUsageAgentData
		err   error
	}
	dockerResult := make(chan dockerUsageResult, 1)
	hostResult := make(chan hostUsageResult, 1)

	go func() {
		usage, err := dockerSdk.Client.DiskUsage(ctx, types.DiskUsageOptions{
			Types: []types.DiskUsageObject{
				types.ContainerObject,
				types.ImageObject,
				types.VolumeObject,
				types.BuildCacheObject,
			},
		})
		if err == nil {
			for i := range usage.Containers {
				usage.Containers[i].Labels = make(map[string]string)
			}
			for i := range usage.Images {
				usage.Images[i].Labels = make(map[string]string)
			}
			for i := range usage.Volumes {
				usage.Volumes[i].Labels = make(map[string]string)
			}
			sort.Slice(usage.Images, func(i, j int) bool {
				return usage.Images[i].Size > usage.Images[j].Size
			})
			sort.Slice(usage.Containers, func(i, j int) bool {
				return usage.Containers[i].SizeRw+usage.Containers[i].SizeRootFs > usage.Containers[j].SizeRw+usage.Containers[j].SizeRootFs
			})
			sort.Slice(usage.Volumes, func(i, j int) bool {
				if usage.Volumes[i].UsageData != nil && usage.Volumes[j].UsageData != nil {
					return usage.Volumes[i].UsageData.Size > usage.Volumes[j].UsageData.Size
				}
				return false
			})
		}
		dockerResult <- dockerUsageResult{usage: usage, err: err}
	}()

	go func() {
		explorer, err := plugin.NewHostExplorer(ctx, dockerSdk)
		if err != nil {
			hostResult <- hostUsageResult{err: fmt.Errorf("initialize host file explorer proxy: %w", err)}
			return
		}
		defer explorer.Close()
		info, err := dockerSdk.Client.Info(ctx)
		if err != nil {
			hostResult <- hostUsageResult{err: fmt.Errorf("get DockerRootDir: %w", err)}
			return
		}
		if info.DockerRootDir == "" {
			hostResult <- hostUsageResult{err: errors.New("DockerRootDir is empty")}
			return
		}

		output, err := dockerSdk.ContainerExecResult(ctx, explorer.ContainerName(), container.ExecOptions{
			Cmd: []string{
				"/agent", "usage",
				"--root", plugin.HostExplorerMountPath,
				"--path", info.DockerRootDir,
			},
		})
		if err != nil {
			hostResult <- hostUsageResult{err: fmt.Errorf("execute agent usage: %w", err)}
			return
		}
		message := diskUsageAgentMessage{}
		decoder := json.NewDecoder(strings.NewReader(output))
		decoder.DisallowUnknownFields()
		if err = decoder.Decode(&message); err != nil {
			hostResult <- hostUsageResult{err: fmt.Errorf("decode agent response: %w", err)}
			return
		}
		var trailing any
		if err = decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
			hostResult <- hostUsageResult{err: errors.New("agent response contains trailing data")}
			return
		}
		if message.Code != 200 {
			if message.Error != "" {
				hostResult <- hostUsageResult{err: errors.New(message.Error)}
				return
			}
			hostResult <- hostUsageResult{err: fmt.Errorf("agent returned code %d", message.Code)}
			return
		}
		if message.Error != "" {
			hostResult <- hostUsageResult{err: fmt.Errorf("agent returned an error with success code: %s", message.Error)}
			return
		}
		if message.Data == nil {
			hostResult <- hostUsageResult{err: errors.New("agent response data is empty")}
			return
		}
		hostResult <- hostUsageResult{usage: *message.Data}
	}()

	succeeded := false
	errs := make([]error, 0, 2)
	for completed := 0; completed < 2; completed++ {
		select {
		case collected := <-dockerResult:
			if collected.err != nil {
				errs = append(errs, fmt.Errorf("collect Docker disk usage: %w", collected.err))
				continue
			}
			result.Usage = &collected.usage
			succeeded = true
		case collected := <-hostResult:
			if collected.err != nil {
				errs = append(errs, fmt.Errorf("collect host disk usage: %w", collected.err))
				continue
			}
			result.HostUsed = &collected.usage.Used
			result.HostAvailable = &collected.usage.Available
			result.HostTotal = &collected.usage.Total
			succeeded = true
		case <-ctx.Done():
			errs = append(errs, ctx.Err())
			completed = 2
		}
	}
	if succeeded {
		result.UpdatedAt = time.Now()
	}
	return result, errors.Join(errs...)
}
