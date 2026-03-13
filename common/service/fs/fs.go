package fs

import (
	"fmt"
	"path"

	"github.com/donknap/dpanel/common/service/compose"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/fs/dockerfs"
	"github.com/donknap/dpanel/common/service/plugin"
	"github.com/spf13/afero"
)

type CreateFsOption struct {
	TargetContainerName string
	TargetVolume        string
	AttachVolumes       []string
	WorkingDir          string // 缺省默认目录
}

func NewContainerFs(dockerSdk *docker.Client, option CreateFsOption) (*afero.Afero, error) {
	var explorer *plugin.Plugin
	var err error

	if option.TargetVolume != "" {
		explorer, err = plugin.NewPlugin(dockerSdk, plugin.ExplorerName, plugin.CreateOption{
			WorkingDir: option.TargetVolume,
			ExtService: compose.ExtService{
				External: compose.ExternalItem{
					Volumes: option.AttachVolumes,
				},
			},
		})
		if err != nil {
			return nil, err
		}
	} else {
		explorer, err = plugin.NewPlugin(dockerSdk, plugin.ExplorerName, plugin.CreateOption{
			Volumes: option.AttachVolumes,
		})
	}

	if err != nil {
		return nil, err
	}
	err = explorer.Create()
	if err != nil {
		return nil, err
	}

	if option.TargetContainerName == "" {
		option.TargetContainerName = plugin.ExplorerName
	}

	containerInfo, err := docker.Sdk.Client.ContainerInspect(dockerSdk.Ctx, option.TargetContainerName)
	if err != nil {
		return nil, err
	}

	if containerInfo.State.Pid == 0 {
		return nil, fmt.Errorf("the %s container does not exist or is not running", option.TargetContainerName)
	}

	dfs, err := dockerfs.New(
		dockerfs.WithDockerSdk(dockerSdk),
		dockerfs.WithProxyContainer(plugin.ExplorerName),
		dockerfs.WithWorkingDir(option.WorkingDir),
		dockerfs.WithTargetContainer(option.TargetContainerName, path.Join(fmt.Sprintf("/proc/%d/root", containerInfo.State.Pid))),
	)

	if err != nil {
		return nil, err
	}

	return &afero.Afero{
		Fs: dfs,
	}, nil
}
