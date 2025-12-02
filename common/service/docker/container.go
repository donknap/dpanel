package docker

import (
	"archive/tar"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/versions"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker/imports"
	"github.com/donknap/dpanel/common/types/define"
)

// ContainerByField 获取单条容器 field 支持 id,name
func (self Client) ContainerByField(ctx context.Context, field string, name ...string) (result map[string]*container.Summary, err error) {
	if len(name) == 0 {
		return nil, errors.New("please specify a container name")
	}
	filtersArgs := filters.NewArgs()

	for _, value := range name {
		filtersArgs.Add(field, value)
	}

	filtersArgs.Add("status", "created")
	filtersArgs.Add("status", "restarting")
	filtersArgs.Add("status", "running")
	filtersArgs.Add("status", "removing")
	filtersArgs.Add("status", "paused")
	filtersArgs.Add("status", "exited")
	filtersArgs.Add("status", "dead")

	containerList, err := self.Client.ContainerList(ctx, container.ListOptions{
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
		if field == "name" {
			key = strings.Trim(temp.Names[0], "/")
		} else if field == "id" {
			key = value.ID
		} else {
			key = value.ID
		}
		result[key] = &temp
	}
	return result, nil
}

func (self Client) ContainerImport(ctx context.Context, containerName string, importFile *imports.ImportFile) error {
	err := self.Client.CopyToContainer(ctx,
		containerName,
		"/",
		importFile.Reader(),
		container.CopyToContainerOptions{},
	)
	defer func() {
		importFile.Close()
	}()
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
	if versions.LessThanOrEqualTo(self.Client.ClientVersion(), "1.44") {
		macAddress := ""
		for name, settings := range info.NetworkSettings.Networks {
			if settings.MacAddress != "" {
				macAddress = settings.MacAddress
				info.NetworkSettings.Networks[name].MacAddress = ""
			}
		}
		if macAddress != "" {
			// 底版本的 docker 需要兼容这一项
			info.Config.MacAddress = macAddress
		}
	}
	return info, nil
}

// ExecResult 在容器中执行一条命令，返回结果
func (self Client) ContainerExecResult(ctx context.Context, containerName string, cmd string) (string, error) {
	execConfig := container.ExecOptions{
		Privileged:   true,
		Tty:          false,
		AttachStdin:  false,
		AttachStdout: true,
		AttachStderr: false,
		Cmd: []string{
			"/bin/sh",
			"-c",
			cmd,
		},
	}
	slog.Debug("command", "exec", []string{
		"/bin/sh",
		"-c",
		cmd,
	})
	response, err := self.ContainerExec(ctx, containerName, execConfig)
	if err != nil {
		return "", err
	}
	defer response.Close()

	stdout, stderr, err := function.SplitStdout(response.Reader)
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
	slog.Debug("docker exec", "command", option)
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
