package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/errdefs"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
)

type ComposeRemoveOption struct {
	CleanNetwork    bool
	CleanVolume     bool
	CleanImage      bool
	ServiceNameList []string
	Output          io.Writer
}

// ComposeRemove 删除一个真实项目下的目标容器；附属资源仅尝试清理，不影响删除结果。
func (self Client) ComposeRemove(ctx context.Context, projectName string, option ComposeRemoveOption) error {
	if !function.IsSafeString(projectName, function.StringRuleName) {
		return function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted)
	}
	for _, name := range option.ServiceNameList {
		if !function.IsSafeString(name, function.StringRuleName) {
			return function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted)
		}
	}
	lock := storage.NewMutex(fmt.Sprintf("compose:remove:%q:%q:%q", self.Client.DaemonHost(), self.Name, projectName))
	if !lock.TryLock() {
		return errors.New("compose removal is already in progress")
	}
	defer lock.Unlock()
	output := option.Output
	if output == nil {
		output = io.Discard
	}
	projectFilter := filters.NewArgs(filters.Arg("label", define.ComposeLabelProject+"="+projectName))
	containerList, err := self.Client.ContainerList(ctx, container.ListOptions{All: true, Filters: projectFilter})
	if err != nil {
		return fmt.Errorf("list compose %q containers: %w", projectName, err)
	}
	containerRemoveList := make([]container.Summary, 0, len(containerList))
	imageMap := make(map[string]bool)
	volumeMap := make(map[string]bool)
	for _, item := range containerList {
		if item.Labels[define.ComposeLabelProject] != projectName {
			return function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted)
		}
		if len(option.ServiceNameList) > 0 && !slices.Contains(option.ServiceNameList, item.Labels[define.ComposeLabelService]) {
			continue
		}
		containerRemoveList = append(containerRemoveList, item)
		if option.CleanImage {
			imageMap[item.ImageID] = true
		}
		if option.CleanVolume {
			for _, mount := range item.Mounts {
				if mount.Type == "volume" {
					volumeMap[mount.Name] = true
				}
			}
		}
	}
	var removeErr error
	for _, item := range containerRemoveList {
		_, _ = fmt.Fprintf(output, "Container %s Stopping\n", item.ID)
		if err := self.Client.ContainerStop(ctx, item.ID, container.StopOptions{}); err != nil && !errdefs.IsNotFound(err) && !errdefs.IsNotModified(err) {
			_, _ = fmt.Fprintf(output, "Container %s: %v\n", item.ID, err)
			removeErr = errors.Join(removeErr, fmt.Errorf("stop compose container %s: %w", item.ID, err))
			continue
		}
		if err := self.Client.ContainerRemove(ctx, item.ID, container.RemoveOptions{RemoveVolumes: option.CleanVolume}); err != nil && !errdefs.IsNotFound(err) {
			_, _ = fmt.Fprintf(output, "Container %s: %v\n", item.ID, err)
			removeErr = errors.Join(removeErr, fmt.Errorf("remove compose container %s: %w", item.ID, err))
			continue
		}
		_, _ = fmt.Fprintf(output, "Container %s Removed\n", item.ID)
	}
	// 按服务删除时保留项目网络，避免影响未选中的服务。
	if option.CleanNetwork && len(option.ServiceNameList) == 0 {
		networkList, err := self.Client.NetworkList(ctx, network.ListOptions{Filters: projectFilter})
		if err != nil {
			_, _ = fmt.Fprintf(output, "Network cleanup: %v\n", err)
		} else {
			for _, item := range networkList {
				if item.Labels[define.ComposeLabelProject] != projectName {
					continue
				}
				if err := self.Client.NetworkRemove(ctx, item.ID); err != nil && !errdefs.IsNotFound(err) {
					_, _ = fmt.Fprintf(output, "Network %s cleanup: %v\n", item.Name, err)
					continue
				}
				_, _ = fmt.Fprintf(output, "Network %s Removed\n", item.Name)
			}
		}
	}
	if option.CleanVolume {
		volumeResponse, err := self.Client.VolumeList(ctx, volume.ListOptions{Filters: projectFilter})
		if err != nil {
			_, _ = fmt.Fprintf(output, "Volume cleanup: %v\n", err)
		} else {
			for _, warning := range volumeResponse.Warnings {
				_, _ = fmt.Fprintf(output, "Volume cleanup: %s\n", warning)
			}
			for _, item := range volumeResponse.Volumes {
				// 外部命名卷即使被目标容器挂载，也不属于本项目的清理范围。
				if item == nil || item.Labels[define.ComposeLabelProject] != projectName {
					continue
				}
				if len(option.ServiceNameList) > 0 && !volumeMap[item.Name] {
					continue
				}
				if err := self.Client.VolumeRemove(ctx, item.Name, false); err != nil && !errdefs.IsNotFound(err) {
					_, _ = fmt.Fprintf(output, "Volume %s cleanup: %v\n", item.Name, err)
					continue
				}
				_, _ = fmt.Fprintf(output, "Volume %s Removed\n", item.Name)
			}
		}
	}
	if option.CleanImage {
		imageList, err := self.Client.ImageList(ctx, image.ListOptions{All: true, Filters: projectFilter})
		if err != nil {
			_, _ = fmt.Fprintf(output, "Image cleanup: %v\n", err)
		} else {
			for _, item := range imageList {
				if item.Labels[define.ComposeLabelProject] == projectName &&
					(len(option.ServiceNameList) == 0 || slices.Contains(option.ServiceNameList, item.Labels[define.ComposeLabelService])) {
					imageMap[item.ID] = true
				}
			}
		}
		for id := range imageMap {
			if _, err := self.Client.ImageRemove(ctx, id, image.RemoveOptions{}); err != nil && !errdefs.IsNotFound(err) {
				_, _ = fmt.Fprintf(output, "Image %s cleanup: %v\n", id, err)
				continue
			}
			_, _ = fmt.Fprintf(output, "Image %s Removed\n", id)
		}
	}
	return removeErr
}
