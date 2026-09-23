package plugin

import "github.com/donknap/dpanel/common/service/docker"

func NewHostExplorer(dockerSkd *docker.Client) (*Plugin, error) {
	explorerPlugin, err := NewPlugin(dockerSkd, ExplorerName, CreateOption{
		Init:                     true,
		RandomProxyContainerName: true,
		MountHostRoot:            true,
	})
	if err != nil {
		return nil, err
	}
	err = explorerPlugin.Create()
	if err != nil {
		_ = explorerPlugin.Close()
		return nil, err
	}
	return explorerPlugin, nil
}
