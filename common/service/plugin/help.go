package plugin

import (
	"context"

	"github.com/donknap/dpanel/common/service/docker"
)

func NewHostExplorer(ctx context.Context, dockerSkd *docker.Client) (*Plugin, error) {
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
	go func() {
		<-ctx.Done()
		_ = explorerPlugin.Close()
	}()
	return explorerPlugin, nil
}
