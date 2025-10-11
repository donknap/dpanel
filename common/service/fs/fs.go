package fs

import (
	"fmt"

	logic2 "github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/fs/dockerfs"
	"github.com/donknap/dpanel/common/service/plugin"
	"github.com/donknap/dpanel/common/service/ssh"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/spf13/afero"
	"github.com/spf13/afero/sftpfs"
)

func NewContainerExplorer(containerName string) (*afero.Afero, error) {
	o := &afero.Afero{}

	// todo 判断一下是否可以直接使用主机 sftp ，从而替换掉代理容器

	explorerPlugin, err := plugin.NewPlugin(plugin.PluginExplorer, nil)
	if err != nil {
		return nil, err
	}
	pluginName, err := explorerPlugin.Create()
	if err != nil {
		return nil, err
	}

	containerInfo, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, containerName)
	if err != nil {
		return nil, err
	}
	if containerInfo.State.Pid == 0 {
		return nil, fmt.Errorf("the %s container does not exist or is not running", containerName)
	}

	dfs, err := dockerfs.New(
		dockerfs.WithDockerSdk(docker.Sdk),
		dockerfs.WithProxyContainer(pluginName),
		dockerfs.WithTargetContainer(containerName, fmt.Sprintf("/proc/%d/root", containerInfo.State.Pid)),
	)
	if err != nil {
		return nil, err
	}
	o.Fs = dfs
	return o, nil
}

func NewSshExplorer(dockerEnvName string) (*ssh.Client, *afero.Afero, error) {
	dockerEnv, err := logic2.DockerEnv{}.GetEnvByName(dockerEnvName)
	if err != nil {
		return nil, nil, err
	}
	if !dockerEnv.EnableSSH || dockerEnv.SshServerInfo == nil {
		return nil, nil, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted)
	}
	option := []ssh.Option{
		ssh.WithSftpClient(),
	}
	option = append(option, ssh.WithServerInfo(dockerEnv.SshServerInfo)...)
	sshClient, err := ssh.NewClient(option...)
	if err != nil {
		return nil, nil, err
	}
	afs := &afero.Afero{
		Fs: sftpfs.New(sshClient.SftpConn),
	}
	return sshClient, afs, nil
}
