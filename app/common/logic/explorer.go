package logic

import (
	"context"
	"errors"
	"strings"

	"github.com/donknap/dpanel/common/function"
	serviceSsh "github.com/donknap/dpanel/common/service/ssh"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/spf13/afero"
	"github.com/spf13/afero/sftpfs"
)

const (
	ExplorerMountTypeDocker = "docker"
	ExplorerMountTypeLocal  = "local"
	ExplorerMountHost       = "host"
	ExplorerMountDPanel     = "dpanel"
)

type Explorer struct {
}

type AfsCreateOption struct {
	Init       bool   // 仅用于与 app/explorer 保持一致的调用风格
	MountPoint string // docker:xxx local:host local:dpanel
}

func (self Explorer) Afs(ctx context.Context, option AfsCreateOption) (*serviceSsh.Client, *afero.Afero, error) {
	mountType, mountName, ok := strings.Cut(option.MountPoint, ":")
	if !ok {
		return nil, nil, errors.New("invalid mount point")
	}

	switch mountType {
	case ExplorerMountTypeLocal:
		switch mountName {
		case ExplorerMountDPanel:
			rootPath := "/dpanel"
			if !function.IsRunInDocker() {
				rootPath = storage.Local{}.GetStorageLocalPath()
			}
			return nil, &afero.Afero{
				Fs: afero.NewBasePathFs(afero.NewOsFs(), rootPath),
			}, nil
		case ExplorerMountHost:
			if !function.IsRunInDocker() {
				return nil, &afero.Afero{
					Fs: afero.NewBasePathFs(afero.NewOsFs(), "/"),
				}, nil
			}
			return self.newSshAfs(ctx, define.DockerDefaultClientName)
		default:
			return nil, nil, errors.New("invalid local mount point")
		}
	case ExplorerMountTypeDocker:
		if !function.IsRunInDocker() && mountName == define.DockerDefaultClientName {
			return nil, &afero.Afero{
				Fs: afero.NewBasePathFs(afero.NewOsFs(), "/"),
			}, nil
		}
		return self.newSshAfs(ctx, mountName)
	default:
		return nil, nil, errors.New("invalid mount type")
	}
}

func (self Explorer) newSshAfs(ctx context.Context, dockerEnvName string) (*serviceSsh.Client, *afero.Afero, error) {
	dockerEnv, err := Env{}.GetEnvByName(dockerEnvName)
	if err != nil {
		return nil, nil, err
	}
	if !dockerEnv.EnableSSH || dockerEnv.SshServerInfo == nil {
		return nil, nil, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted)
	}

	option := []serviceSsh.Option{
		serviceSsh.WithContext(ctx),
		serviceSsh.WithSftpClient(),
	}
	option = append(option, serviceSsh.WithServerInfo(dockerEnv.SshServerInfo)...)
	sshClient, err := serviceSsh.NewClient(option...)
	if err != nil {
		return nil, nil, err
	}

	go func() {
		<-ctx.Done()
		sshClient.Close()
	}()

	return sshClient, &afero.Afero{
		Fs: sftpfs.New(sshClient.SftpConn),
	}, nil
}
