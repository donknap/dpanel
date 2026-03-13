package logic

import (
	"context"

	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/ssh"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/spf13/afero"
	"github.com/spf13/afero/sftpfs"
)

type Explorer struct {
}

func (self Explorer) Afs(ctx context.Context, dockerEnvName string) (*ssh.Client, *afero.Afero, error) {
	dockerEnv, err := Env{}.GetEnvByName(dockerEnvName)
	if err != nil {
		return nil, nil, err
	}
	if !dockerEnv.EnableSSH || dockerEnv.SshServerInfo == nil {
		return nil, nil, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted)
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
	go func() {
		<-ctx.Done()
		sshClient.Close()
	}()
	afs := &afero.Afero{
		Fs: sftpfs.New(sshClient.SftpConn),
	}
	return sshClient, afs, nil
}
