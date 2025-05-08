package explorer

import (
	"context"
	"errors"
	"fmt"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/plugin"
)

type Option func(self *explorer) error

func WithRootPath(path string) Option {
	return func(self *explorer) error {
		self.rootPath = path
		return nil
	}
}

func WithRootPathFromContainer(md5 string) Option {
	return func(self *explorer) error {
		containerInfo, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, md5)
		if err != nil {
			return err
		}
		if containerInfo.State.Pid == 0 {
			return errors.New("please start the container" + md5)
		}
		return WithRootPath(fmt.Sprintf("/proc/%d/root", containerInfo.State.Pid))(self)
	}
}

func WithProxyContainerRunner() Option {
	return func(self *explorer) error {
		explorerPlugin, err := plugin.NewPlugin(plugin.PluginExplorer, nil)
		if err != nil {
			return err
		}
		pluginName, err := explorerPlugin.Create()
		if err != nil {
			return err
		}
		return WithRunContainer(pluginName)(self)
	}
}

func WithRunContainer(name string) Option {
	return func(self *explorer) error {
		self.runner = func(cmd string) (string, error) {
			ctx, cancel := context.WithCancel(docker.Sdk.Ctx)
			defer func() {
				cancel()
			}()
			return docker.Sdk.ExecResult(ctx, name, cmd)
		}
		return nil
	}
}
