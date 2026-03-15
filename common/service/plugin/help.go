package plugin

import (
	"context"

	types2 "github.com/compose-spec/compose-go/v2/types"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/types"
)

func NewHostExplorer(ctx context.Context, dockerSkd *docker.Client) (string, error) {
	explorerPlugin, err := NewPlugin(dockerSkd, ExplorerName, CreateOption{
		Volumes: []types.VolumeItem{
			{
				Dest: "/mnt/host",
				Host: "/",
				Type: types2.VolumeTypeBind,
			},
		},
		RandomProxyContainerName: true,
	})
	if err != nil {
		return "", err
	}
	err = explorerPlugin.Create()
	if err != nil {
		return "", err
	}
	go func() {
		<-ctx.Done()
		_ = explorerPlugin.Close()
	}()
	return explorerPlugin.containerName, nil
}
