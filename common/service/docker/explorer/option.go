package explorer

import (
	"errors"
	"fmt"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/plugin"
)

type Option func(self *explorer) error

func WithRootPathFromContainer(md5 string) Option {
	return func(self *explorer) error {
		containerInfo, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, md5)
		if err != nil {
			return err
		}
		if containerInfo.State.Pid == 0 {
			return errors.New("please start the container" + md5)
		}
		self.rootPath = fmt.Sprintf("/proc/%d/root", containerInfo.State.Pid)
		return nil
	}
}

func WithRootPath(path string) Option {
	return func(self *explorer) error {
		self.rootPath = path
		return nil
	}
}

func WithProxyPlugin() Option {
	return func(self *explorer) error {
		explorerPlugin, err := plugin.NewPlugin(plugin.PluginExplorer, nil)
		if err != nil {
			return err
		}
		pluginName, err := explorerPlugin.Create()
		if err != nil {
			return err
		}
		self.runContainer = pluginName
		return nil
	}
}

func WithRunContainer(name string) Option {
	return func(self *explorer) error {
		self.runContainer = name
		return nil
	}
}
