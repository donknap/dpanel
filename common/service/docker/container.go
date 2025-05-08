package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/versions"
	"github.com/donknap/dpanel/common/function"
	"io"
	"log/slog"
	"os"
	"strings"
)

// ContainerByField 获取单条容器 field 支持 id,name
func (self Builder) ContainerByField(ctx context.Context, field string, name ...string) (result map[string]*container.Summary, err error) {
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

	containerList, err := Sdk.Client.ContainerList(ctx, container.ListOptions{
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

func (self Builder) ContainerImport(ctx context.Context, containerName string, file *ImportFile) error {
	if err := self.Client.CopyToContainer(ctx,
		containerName,
		"/",
		file.Reader,
		container.CopyToContainerOptions{},
	); err != nil {
		return err
	}
	return nil
}

// ContainerCopyInspect 获取复制容器信息，兼容低版本的配置情况
func (self Builder) ContainerCopyInspect(ctx context.Context, containerName string) (info container.InspectResponse, err error) {
	info, err = Sdk.Client.ContainerInspect(ctx, containerName)
	if err != nil {
		return info, err
	}
	return self.ContainerInspectCompat(info)
}

func (self Builder) ContainerInspectCompat(info container.InspectResponse) (container.InspectResponse, error) {
	if versions.LessThanOrEqualTo(Sdk.Client.ClientVersion(), "1.44") {
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
func (self Builder) ExecResult(ctx context.Context, containerName string, cmd string) (string, error) {
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
	response, err := Sdk.ContainerExec(ctx, containerName, execConfig)
	if err != nil {
		return "", err
	}
	defer response.Close()

	buffer := new(bytes.Buffer)
	_, err = io.Copy(buffer, response.Reader)
	if err != nil {
		return "", err
	}
	cleanOut := self.ExecCleanResult(buffer.Bytes())
	slog.Debug("command", "clear result", cleanOut)
	return cleanOut, nil
}

// ContainerExec 在容器内执行一条 shell 命令
func (self Builder) ContainerExec(ctx context.Context, containerName string, option container.ExecOptions) (types.HijackedResponse, error) {
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
func (self Builder) ContainerReadFile(ctx context.Context, containerName string, inContainerPath string, targetFile *os.File) (io.Reader, error) {
	pathStat, err := self.Client.ContainerStatPath(ctx, containerName, inContainerPath)
	if err != nil {
		return nil, err
	}
	if !pathStat.Mode.IsRegular() {
		return nil, function.ErrorMessage(".containerExplorerContentUnsupportedType")
	}
	out, _, err := self.Client.CopyFromContainer(ctx, containerName, inContainerPath)
	if err != nil {
		return nil, err
	}
	tarReader := tar.NewReader(out)
	defer func() {
		_ = out.Close()
	}()

	_, err = tarReader.Next()
	if err != nil {
		return nil, err
	}
	if targetFile != nil {
		_, err = io.Copy(targetFile, tarReader)
		if err != nil {
			return nil, err
		}
		return targetFile, nil
	} else {
		buffer := new(bytes.Buffer)
		_, err = io.Copy(buffer, tarReader)
		if err != nil {
			return nil, err
		}
		return buffer, nil
	}
}
