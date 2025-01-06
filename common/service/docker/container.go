package docker

import (
	"archive/tar"
	"bytes"
	"errors"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/versions"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// 获取单条容器 field 支持 id,name
func (self Builder) ContainerByField(field string, name ...string) (result map[string]*types.Container, err error) {
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

	containerList, err := Sdk.Client.ContainerList(Sdk.Ctx, container.ListOptions{
		Filters: filtersArgs,
	})
	if err != nil {
		return nil, err
	}
	if len(containerList) == 0 {
		return nil, errors.New("container not found")
	}
	result = make(map[string]*types.Container)

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

func (self Builder) ContainerInfo(md5 string) (info types.ContainerJSON, err error) {
	info, _, err = Sdk.Client.ContainerInspectWithRaw(Sdk.Ctx, md5, true)
	if err != nil {
		return info, err
	}
	info.Name = strings.TrimPrefix(info.Name, "/")
	return info, nil
}

func (self Builder) ContainerCopyContentIn(containerName, fileName, content string, perm os.FileMode) error {
	buf := new(bytes.Buffer)
	tarWriter := tar.NewWriter(buf)
	defer func() {
		_ = tarWriter.Close()
	}()
	if err := tarWriter.WriteHeader(&tar.Header{
		Name:    fileName,
		Size:    int64(len(content)),
		Mode:    int64(perm),
		ModTime: time.Now(),
	}); err != nil {
		return err
	}
	if _, err := tarWriter.Write([]byte(content)); err != nil {
		return err
	}

	if err := self.Client.CopyToContainer(self.Ctx,
		containerName,
		"/",
		buf,
		container.CopyToContainerOptions{},
	); err != nil {
		return err
	}
	return nil
}

func (self Builder) ContainerCopyPathIn(containerName, containerDestPath string, file []string) error {
	buf := new(bytes.Buffer)
	tarWriter := tar.NewWriter(buf)
	defer func() {
		_ = tarWriter.Close()
	}()
	for _, item := range file {
		sourceFile, err := os.Open(item)
		if err != nil {
			return err
		}
		fileInfo, _ := sourceFile.Stat()
		if err := tarWriter.WriteHeader(&tar.Header{
			Name:    filepath.Base(item),
			Size:    fileInfo.Size(),
			Mode:    int64(fileInfo.Mode()),
			ModTime: fileInfo.ModTime(),
		}); err != nil {
			return err
		}
		content, err := io.ReadAll(sourceFile)
		if _, err := tarWriter.Write(content); err != nil {
			return err
		}
		_ = sourceFile.Close()
	}

	if err := self.Client.CopyToContainer(self.Ctx,
		containerName,
		containerDestPath,
		buf,
		container.CopyToContainerOptions{},
	); err != nil {
		return err
	}
	return nil
}

// 获取复制容器信息，兼容低版本的配置情况
func (self Builder) ContainerCopyInspect(containerName string) (info types.ContainerJSON, err error) {
	info, err = Sdk.Client.ContainerInspect(Sdk.Ctx, containerName)
	if err != nil {
		return info, err
	}
	if versions.LessThanOrEqualTo(Sdk.Client.ClientVersion(), "1.44") {
		macAddress := ""
		for name, settings := range info.NetworkSettings.Networks {
			if settings.MacAddress != "" {
				macAddress = settings.MacAddress
				info.NetworkSettings.Networks[name].MacAddress = ""
			}
		}
		if macAddress != "" {
			info.Config.MacAddress = macAddress
		}
	}
	return info, nil
}
