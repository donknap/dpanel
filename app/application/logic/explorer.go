package logic

import (
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/service/compose"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/fs/dockerfs"
	"github.com/donknap/dpanel/common/service/plugin"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/spf13/afero"
)

const (
	MountTypeContainer = "container"
	MountTypeVolume    = "volume"
)

type Explorer struct {
}

type AfsCreateOption struct {
	Init       bool   // 只有初始化的时候会创建文件管理器
	MountPoint string // container:xxx volume:yyy
}

// Afs 获取文件操作对象
func (self Explorer) Afs(dockerSdk *docker.Client, option AfsCreateOption) (*afero.Afero, error) {
	var pluginOption plugin.CreateOption
	var targetContainerName string
	var mountType, mountTarget string
	if before, after, ok := strings.Cut(option.MountPoint, ":"); ok {
		mountType = before
		mountTarget = after
	} else {
		mountType = MountTypeContainer
		mountTarget = option.MountPoint
	}

	if !option.Init {
		if v, ok := storage.Cache.Get(fmt.Sprintf(storage.CacheKeyExplorerAfs, mountTarget)); ok {
			return v.(*afero.Afero), nil
		}
		return nil, errors.New("file explorer not initialized")
	}

	switch mountType {
	case MountTypeVolume:
		targetContainerName = plugin.ExplorerName
		volumeInfo, err := dockerSdk.Client.VolumeInspect(dockerSdk.Ctx, mountTarget)
		if err != nil {
			return nil, err
		}
		mountPath := path.Join("/", volumeInfo.Name)
		pluginOption = plugin.CreateOption{
			WorkingDir: mountPath,
			ExtService: compose.ExtService{
				External: compose.ExternalItem{
					Volumes: []string{
						fmt.Sprintf("%s:%s", volumeInfo.Name, mountPath),
					},
				},
			},
		}
	default:
		targetContainerName = mountTarget
		dpanelInfo := logic.Setting{}.GetDPanelInfo()
		if dpanelInfo.Mount.Host != "" {
			pluginOption = plugin.CreateOption{
				Volumes: []types.VolumeItem{
					dpanelInfo.Mount,
				},
			}
		}
	}

	explorer, err := plugin.NewPlugin(dockerSdk, plugin.ExplorerName, pluginOption)
	if err != nil {
		return nil, err
	}

	err = explorer.Create()
	if err != nil {
		return nil, err
	}

	afsOption := []dockerfs.Option{
		dockerfs.WithDockerSdk(dockerSdk),
		dockerfs.WithProxyContainer(plugin.ExplorerName),
	}

	if targetContainerName != plugin.ExplorerName {
		containerInfo, err := docker.Sdk.Client.ContainerInspect(dockerSdk.Ctx, mountTarget)
		if err != nil {
			return nil, err
		}

		if containerInfo.State.Pid == 0 {
			return nil, fmt.Errorf("the %s container does not exist or is not running", mountTarget)
		}

		afsOption = append(afsOption,
			dockerfs.WithWorkingDir(containerInfo.Config.WorkingDir),
			dockerfs.WithTargetContainer(mountTarget, path.Join(fmt.Sprintf("/proc/%d/root", containerInfo.State.Pid))),
		)
	} else {
		afsOption = append(afsOption,
			dockerfs.WithTargetContainer(plugin.ExplorerName, "/"),
		)
	}

	dfs, err := dockerfs.New(afsOption...)

	if err != nil {
		return nil, err
	}

	afs := &afero.Afero{
		Fs: dfs,
	}

	storage.Cache.Set(fmt.Sprintf(storage.CacheKeyExplorerAfs, targetContainerName), afs, 60*time.Second)

	return afs, nil
}
