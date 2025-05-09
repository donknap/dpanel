package controller

import (
	"bytes"
	logic2 "github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/explorer"
	"github.com/donknap/dpanel/common/service/ssh"
	"github.com/gin-gonic/gin"
	"github.com/pkg/sftp"
	"github.com/spf13/afero"
	"github.com/spf13/afero/sftpfs"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"io"
	"os"
	"path/filepath"
	"strconv"
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
	result := make([]*explorer.FileData, 0)
	for _, item := range list {
		fileData := &explorer.FileData{
			Name:     item.Name(),
			Mod:      item.Mode(),
			ModTime:  item.ModTime(),
			Change:   explorer.ChangeDefault,
			Size:     item.Size(),
			Owner:    "",
			Group:    "",
			LinkName: "",
		}
		if v, ok := item.Sys().(*sftp.FileStat); ok {
			fileData.Group = strconv.Itoa(int(v.GID))
			fileData.Owner = strconv.Itoa(int(v.UID))
		}
		if fileData.IsSymlink() {
			if linkName, err := sftpClient.ReadLink(filepath.Join(params.Path, item.Name())); err == nil {
				fileData.LinkName = linkName
			}
		}
		result = append(result, fileData)
	}
	self.JsonResponseWithoutError(http, gin.H{
		"list": result,
	})
	return
}

func (self Explorer) GetUserList(http *gin.Context) {
	type ParamsValidate struct {
		Name string `json:"name" binding:"required"`
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
	buffer := new(bytes.Buffer)
	if file, err := sftpClient.OpenFile("/etc/passwd", os.O_RDONLY); err == nil {
		defer func() {
			_ = file.Close()
		}()
		_, _ = io.Copy(buffer, file)
	}
	self.JsonResponseWithoutError(http, gin.H{
		"content": buffer.String(),
	})
	return
}
