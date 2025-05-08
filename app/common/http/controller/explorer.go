package controller

import (
	"fmt"
	logic2 "github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/ssh"
	"github.com/gin-gonic/gin"
	"github.com/pkg/sftp"
	"github.com/spf13/afero"
	"github.com/spf13/afero/sftpfs"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type Explorer struct {
	controller.Abstract
}

func (self Explorer) GetPathList(http *gin.Context) {
	type ParamsValidate struct {
		Name string `json:"name" binding:"required"`
		Path string `json:"path" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	dockerEnv, err := logic2.DockerEnv{}.GetEnvByName(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if !dockerEnv.EnableSSH || dockerEnv.SshServerInfo == nil {
		self.JsonResponseWithError(http, function.ErrorMessage(".commonDataNotFoundOrDeleted"), 500)
		return
	}
	sshClient, err := ssh.NewClient(ssh.WithServerInfo(dockerEnv.SshServerInfo)...)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		sshClient.Close()
	}()
	sftpClient, err := sftp.NewClient(sshClient.Conn)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	afs := &afero.Afero{
		Fs: sftpfs.New(sftpClient),
	}
	list, err := afs.ReadDir(params.Path)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	fmt.Printf("%v \n", list)
	self.JsonResponseWithoutError(http, gin.H{
		"list": make([]string, 0),
	})
	return

}
