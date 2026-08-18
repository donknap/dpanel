package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/types/define"
)

const (
	ContainerFilterID       = "id"
	ContainerFilterName     = "name"
	ContainerFilterStatus   = "status"
	ContainerFilterLabel    = "label"
	ContainerFilterAncestor = "ancestor"
)

const (
	ContainerFilterExtendPrefix = "x-"

	ContainerFilterNameContains  = "x-name-contains"
	ContainerFilterImageContains = "x-image-contains"
	ContainerFilterCompose       = "x-compose"
)

func (self Client) ContainerSearchList(ctx context.Context, option container.ListOptions) ([]container.Summary, error) {
	filter := option.Filters.Clone()
	extendFilters := make(map[string][]string)
	for _, key := range filter.Keys() {
		if !strings.HasPrefix(key, ContainerFilterExtendPrefix) {
			continue
		}
		switch key {
		case ContainerFilterNameContains, ContainerFilterImageContains, ContainerFilterCompose:
			extendFilters[key] = filter.Get(key)
			for _, value := range extendFilters[key] {
				filter.Del(key, value)
			}
		default:
			return nil, fmt.Errorf("invalid container extension filter %s", key)
		}
	}

	option.Filters = filter
	if composeValues, ok := extendFilters[ContainerFilterCompose]; ok && len(composeValues) == 1 {
		option.Filters.Add(ContainerFilterLabel, fmt.Sprintf("%s=%s", define.ComposeLabelProject, composeValues[0]))
	}
	limit := option.Limit
	if len(extendFilters) > 0 && limit > 0 {
		option.Limit = 0
	}
	list, err := self.Client.ContainerList(ctx, option)
	if err != nil {
		return nil, err
	}
	if len(extendFilters) == 0 {
		return list, nil
	}

	result := make([]container.Summary, 0, len(list))
	for _, item := range list {
		match := true
		for key, values := range extendFilters {
			keyMatch := false
			for _, value := range values {
				// 名称和镜像 contains 的空字符串按既有语义匹配全部；DPanel 强制排除仍由上层在筛选完成后执行。
				switch key {
				case ContainerFilterNameContains:
					for _, name := range item.Names {
						if strings.Contains(strings.TrimPrefix(name, "/"), value) {
							keyMatch = true
							break
						}
					}
				case ContainerFilterImageContains:
					keyMatch = strings.Contains(item.Image, value)
				case ContainerFilterCompose:
					composeProject, exists := item.Labels[define.ComposeLabelProject]
					keyMatch = exists && composeProject == value
				}
				if keyMatch {
					break
				}
			}
			if !keyMatch {
				match = false
				break
			}
		}
		if match {
			result = append(result, item)
		}
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

// ContainerByField 获取单条容器 field 支持 id,name
func (self Client) ContainerByField(ctx context.Context, field string, name ...string) (result map[string]*container.Summary, err error) {
	if len(name) == 0 {
		return nil, errors.New("please specify a container name")
	}
	filtersArgs := filters.NewArgs()

	for _, value := range name {
		filtersArgs.Add(field, value)
	}

	filtersArgs.Add(ContainerFilterStatus, string(container.StateCreated))
	filtersArgs.Add(ContainerFilterStatus, string(container.StateRestarting))
	filtersArgs.Add(ContainerFilterStatus, string(container.StateRunning))
	filtersArgs.Add(ContainerFilterStatus, string(container.StateRemoving))
	filtersArgs.Add(ContainerFilterStatus, string(container.StatePaused))
	filtersArgs.Add(ContainerFilterStatus, string(container.StateExited))
	filtersArgs.Add(ContainerFilterStatus, string(container.StateDead))

	containerList, err := self.ContainerSearchList(ctx, container.ListOptions{
		Filters: filtersArgs,
	})
	if err != nil {
		return nil, err
	}
	if len(containerList) == 0 {
		return nil, errors.New("container not found")
	}
	result = make(map[string]*container.Summary)

	var key string
	for _, value := range containerList {
		temp := value
		if field == ContainerFilterName {
			key = strings.Trim(temp.Names[0], "/")
		} else if field == ContainerFilterID {
			key = value.ID
		} else {
			key = value.ID
		}
		result[key] = &temp
	}
	return result, nil
}

func (self Client) ContainerImport(ctx context.Context, containerName string, dstPath string, reader io.Reader) error {
	err := self.Client.CopyToContainer(ctx,
		containerName,
		dstPath,
		reader,
		container.CopyToContainerOptions{},
	)
	if err != nil {
		return err
	}
	return nil
}

// ContainerCopyInspect 获取复制容器信息，兼容低版本的配置情况
func (self Client) ContainerCopyInspect(ctx context.Context, containerName string) (info container.InspectResponse, err error) {
	info, err = self.Client.ContainerInspect(ctx, containerName)
	if err != nil {
		return info, err
	}
	return self.ContainerInspectCompat(info)
}

func (self Client) ContainerInspectCompat(info container.InspectResponse) (container.InspectResponse, error) {
	if info.Config != nil {
		containerConfig := *info.Config
		if info.Config.Labels != nil {
			containerConfig.Labels = make(map[string]string, len(info.Config.Labels))
			for name, value := range info.Config.Labels {
				containerConfig.Labels[name] = value
			}
		}
		info.Config = &containerConfig
	}
	if info.HostConfig != nil {
		hostConfig := *info.HostConfig
		hostConfig.ExtraHosts = append([]string(nil), info.HostConfig.ExtraHosts...)
		hostConfig.VolumesFrom = append([]string(nil), info.HostConfig.VolumesFrom...)
		// compatible cgroup v2 不支持配置 MemorySwappiness，podman 在 crun 下会严格报错
		hostConfig.MemorySwappiness = nil
		info.HostConfig = &hostConfig
	}
	if info.NetworkSettings != nil {
		networkSettings := *info.NetworkSettings
		if info.NetworkSettings.Networks != nil {
			networkSettings.Networks = make(map[string]*network.EndpointSettings, len(info.NetworkSettings.Networks))
			for name, settings := range info.NetworkSettings.Networks {
				if settings == nil {
					networkSettings.Networks[name] = nil
					continue
				}
				endpointSettings := *settings
				networkSettings.Networks[name] = &endpointSettings
			}
		}
		info.NetworkSettings = &networkSettings
	}
	if versions.LessThanOrEqualTo(self.Client.ClientVersion(), "1.44") {
		macAddress := ""
		if info.NetworkSettings != nil {
			for name, settings := range info.NetworkSettings.Networks {
				if settings != nil && settings.MacAddress != "" {
					macAddress = settings.MacAddress
					info.NetworkSettings.Networks[name].MacAddress = ""
				}
			}
		}
		if macAddress != "" && info.Config != nil {
			// 底版本的 docker 需要兼容这一项
			info.Config.MacAddress = macAddress
		}
	}
	return info, nil
}

// ContainerExecResult 在容器中执行一条命令，返回结果
func (self Client) ContainerExecResult(ctx context.Context, containerName string, cmd string) (string, error) {
	execConfig := container.ExecOptions{
		Privileged:   true,
		Tty:          false,
		AttachStdin:  false,
		AttachStdout: true,
		AttachStderr: true,
		Cmd: []string{
			"/bin/sh",
			"-c",
			cmd,
		},
	}
	slog.Info("command", "exec", []string{
		"/bin/sh",
		"-c",
		cmd,
	})
	response, err := self.ContainerExec(ctx, containerName, execConfig)
	if err != nil {
		return "", err
	}
	defer response.Close()

	var stdout, stderr bytes.Buffer
	_, err = stdcopy.StdCopy(&stdout, &stderr, response.Reader)
	if err != nil {
		return "", err
	}
	if stderr.Len() > 0 {
		return "", errors.New(stderr.String())
	}
	return stdout.String(), nil
}

// ContainerExec 在容器内执行一条 shell 命令
func (self Client) ContainerExec(ctx context.Context, containerName string, option container.ExecOptions) (types.HijackedResponse, error) {
	slog.Info("docker exec", "command", option)
	exec, err := self.Client.ContainerExecCreate(ctx, containerName, option)
	if err != nil {
		return types.HijackedResponse{}, err
	}
	execAttachOption := container.ExecStartOptions{
		Tty:         option.Tty,
		ConsoleSize: option.ConsoleSize,
		Detach:      option.Detach,
	}
	return self.Client.ContainerExecAttach(ctx, exec.ID, execAttachOption)
}

// ContainerReadFile 读取容器内的一个文件内容，传入 targetFile 则写入文件 否则返回一个 reader
func (self Client) ContainerReadFile(ctx context.Context, containerName string, inContainerPath string, targetFile *os.File) (io.ReadCloser, error) {
	pathStat, err := self.Client.ContainerStatPath(ctx, containerName, inContainerPath)
	if err != nil {
		return nil, err
	}
	if !pathStat.Mode.IsRegular() {
		return nil, function.ErrorMessage(define.ErrorMessageContainerExplorerContentUnsupportedType)
	}
	out, _, err := self.Client.CopyFromContainer(ctx, containerName, inContainerPath)
	if err != nil {
		return nil, err
	}
	// 返回的数据是外部是一个 tar 真正的文件 reader 需要先读一次
	tarReader := tar.NewReader(out)
	file, err := tarReader.Next()
	if err != nil {
		return nil, err
	}

	if targetFile == nil {
		return out, nil
	}

	defer func() {
		_ = out.Close()
	}()

	_, err = io.Copy(targetFile, tarReader)
	if err != nil {
		return nil, err
	}

	_ = targetFile.Chmod(file.FileInfo().Mode())
	return nil, nil
}

func (self Client) ContainerLogs(ctx context.Context, containerId string, options container.LogsOptions) (io.ReadCloser, error) {
	inspectInfo, err := self.Client.ContainerInspect(ctx, containerId)
	if err != nil {
		return nil, err
	}
	reader, err := self.Client.ContainerLogs(ctx, containerId, options)
	if err != nil {
		return nil, err
	}
	if inspectInfo.Config.Tty {
		return reader, nil
	}

	return function.DockerCombinedStream(reader), nil
}
