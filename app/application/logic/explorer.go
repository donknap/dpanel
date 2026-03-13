package logic

import (
	"fmt"
	"strings"

	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/fs"
	"github.com/donknap/dpanel/common/service/plugin"
	"github.com/spf13/afero"
)

type Explorer struct {
}

// Afs 获取文件操作对象
// path in container:container /app/test
// path in container:host /Users/xxx/Document
// path in container:volume 136a330b867a4534dd4d20a13f1226579f85542bddbe77aa7c117437b507a6a0
func (self Explorer) Afs(dockerSdk *docker.Client, mountPoint string) (*afero.Afero, string, error) {
	createOption := fs.CreateFsOption{}
	mountType := "container"
	mountTarget := mountPoint

	if b, a, ok := strings.Cut(mountPoint, ":"); ok {
		mountType = b
		mountTarget = a
	}
	switch mountType {
	case "volume":
		volumeInfo, err := dockerSdk.Client.VolumeInspect(dockerSdk.Ctx, mountTarget)
		if err != nil {
			return nil, "", err
		}
		createOption.TargetVolume = volumeInfo.Name
		createOption.TargetContainerName = plugin.ExplorerName
	default:
		containerInfo, err := dockerSdk.Client.ContainerInspect(dockerSdk.Ctx, mountTarget)
		if err != nil {
			return nil, "", err
		}
		dpanelInfo := logic.Setting{}.GetDPanelInfo()
		createOption.TargetContainerName = containerInfo.Name
		createOption.WorkingDir = containerInfo.Config.WorkingDir
		createOption.AttachVolumes = []string{
			fmt.Sprintf("%s:/dpanel", dpanelInfo.MountPath),
		}
	}
	afs, err := fs.NewContainerFs(dockerSdk, createOption)
	return afs, createOption.TargetContainerName, err

}
