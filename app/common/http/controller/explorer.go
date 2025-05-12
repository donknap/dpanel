package controller

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types/container"
	logic2 "github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/explorer"
	"github.com/donknap/dpanel/common/service/ssh"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/gin-gonic/gin"
	"github.com/h2non/filetype"
	"github.com/pkg/sftp"
	"github.com/spf13/afero"
	"github.com/spf13/afero/sftpfs"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Explorer struct {
	controller.Abstract
}

func (self Explorer) Export(http *gin.Context) {
	type ParamsValidate struct {
		Name     string   `json:"name" binding:"required"`
		FileList []string `json:"fileList" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	var err error

	tempFile, err := storage.Local{}.CreateTempFile("")
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
	}()

	sshClient, afs, err := self.getClient(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		sshClient.Close()
	}()

	pathInfo := make([]container.PathStat, 0)
	zipWriter := zip.NewWriter(tempFile)

	for _, path := range params.FileList {
		err = func() error {
			file, err := afs.Open(path)
			if err != nil {
				return err
			}
			defer func() {
				_ = file.Close()
			}()
			fileInfo, err := file.Stat()
			if err != nil {
				return err
			}
			zipFileInfo, err := zip.FileInfoHeader(fileInfo)
			if err != nil {
				return err
			}
			writer, _ := zipWriter.CreateHeader(zipFileInfo)
			_, _ = io.Copy(writer, file)
			return nil
		}()
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	if info, err := json.Marshal(pathInfo); err == nil {
		writer, _ := zipWriter.CreateHeader(&zip.FileHeader{
			Name:               "manifest.json",
			Method:             zip.Deflate,
			UncompressedSize64: uint64(len(info)),
			Modified:           time.Now(),
		})
		_, _ = writer.Write(info)
	}

	err = zipWriter.Close()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	http.Header("Content-Type", "application/zip")
	http.Header("Content-Disposition", "attachment; filename=export.zip")
	http.File(tempFile.Name())
	return
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
	sshClient, afs, err := self.getClient(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		sshClient.Close()
	}()

	list, err := afs.ReadDir(params.Path)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	result := make([]*explorer.FileData, 0)
	for _, item := range list {
		fileData := &explorer.FileData{
			Path:     filepath.Join(params.Path, item.Name()),
			Name:     item.Name(),
			Mod:      item.Mode(),
			ModTime:  item.ModTime(),
			Change:   explorer.ChangeDefault,
			Size:     item.Size(),
			Owner:    "",
			Group:    "",
			LinkName: "",
			IsDir:    item.Mode().IsDir(),
		}
		if v, ok := item.Sys().(*sftp.FileStat); ok {
			cacheKey := fmt.Sprintf(storage.CacheKeyExplorerUsername, params.Name, v.UID)
			if username, ok := storage.Cache.Get(cacheKey); ok {
				fileData.Owner = username.(string)
			}
			if fileData.Owner == "" {
				if username, err := sshClient.Run(fmt.Sprintf("id -un %d", v.UID)); err == nil {
					fileData.Owner = strings.TrimSpace(username)
					_ = storage.Cache.Add(cacheKey, username, time.Hour)
				} else {
					fileData.Owner = strconv.Itoa(int(v.UID))
				}
			}
			fileData.Group = strconv.Itoa(int(v.GID))
		}
		if fileData.CheckIsSymlink() {
			fileData.IsSymlink = true
			if linkName, err := sshClient.SftpConn.ReadLink(filepath.Join(params.Path, item.Name())); err == nil {
				fileData.LinkName = linkName
			}
		}
		result = append(result, fileData)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].IsDir && !result[j].IsDir
	})
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return result[i].Name < result[j].Name
	})
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
	sshClient, afs, err := self.getClient(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		sshClient.Close()
	}()

	groups := make([]map[string]any, 0)
	users := make([]map[string]any, 0)

	if passwd, err := afs.ReadFile("/etc/passwd"); err == nil && string(passwd) != "" {
		users = function.PluckArrayWalk(strings.Split(string(passwd), "\n"), func(line string) (map[string]any, bool) {
			items := strings.Split(line, ":")
			if len(items) < 7 {
				return nil, false
			}
			return map[string]any{
				"name":        items[0],
				"gid":         items[3],
				"uid":         items[2],
				"description": items[4],
			}, true
		})
	} else {
		slog.Debug("explorer get user list", "err", err)
	}

	if group, err := afs.ReadFile("/etc/group"); err == nil && string(group) != "" {
		groups = function.PluckArrayWalk(strings.Split(string(group), "\n"), func(line string) (map[string]any, bool) {
			items := strings.Split(line, ":")
			if len(items) < 3 {
				return nil, false
			}
			return map[string]any{
				"name": items[0],
				"gid":  items[2],
			}, true
		})
	}

	self.JsonResponseWithoutError(http, gin.H{
		"group": groups,
		"user":  users,
	})
	return
}

func (self Explorer) GetContent(http *gin.Context) {
	type ParamsValidate struct {
		Name string `json:"name" binding:"required"`
		File string `json:"file" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	sshClient, afs, err := self.getClient(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		sshClient.Close()
	}()
	file, err := afs.OpenFile(params.File, os.O_RDONLY, 0o644)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		_ = file.Close()
	}()
	fileInfo, err := file.Stat()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if fileInfo.Size() >= 1024*1024 {
		self.JsonResponseWithError(http, function.ErrorMessage(".containerExplorerEditFileMaxSize"), 500)
		return
	}
	fileType, _ := filetype.MatchFile(file.Name())
	if fileType == filetype.Unknown {
		content, err := io.ReadAll(file)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		self.JsonResponseWithoutError(http, gin.H{
			"content": string(content),
		})
		return
	} else {
		self.JsonResponseWithError(http, function.ErrorMessage(".containerExplorerContentUnsupportedType"), 500)
		return
	}
}

func (self Explorer) GetFileStat(http *gin.Context) {
	type ParamsValidate struct {
		Name string `json:"name" binding:"required"`
		Path string `json:"path" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	sshClient, afs, err := self.getClient(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		sshClient.Close()
	}()
	var fileInfo os.FileInfo
	file, err := afs.OpenFile(params.Path, os.O_RDONLY, 0o644)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		_ = file.Close()
	}()
	fileInfo, err = file.Stat()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"info": gin.H{
			"isDir":  fileInfo.Mode().IsDir(),
			"target": params.Path,
			"name":   filepath.Base(params.Path),
		},
	})
	return
}

func (self Explorer) Import(http *gin.Context) {
	type ParamsValidate struct {
		Name     string `json:"name" binding:"required"`
		FileList []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"fileList" binding:"required"`
		DestPath string `json:"destPath" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	defer func() {
		for _, s := range params.FileList {
			realPath := storage.Local{}.GetRealPath(s.Path)
			_ = os.Remove(realPath)
		}
	}()
	sshClient, afs, err := self.getClient(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		sshClient.Close()
	}()

	for _, item := range params.FileList {
		err = func() error {
			realPath, err := os.Open(storage.Local{}.GetRealPath(item.Path))
			if err != nil {
				return err
			}
			defer func() {
				_ = realPath.Close()
			}()
			err = afs.WriteReader(filepath.Join(params.DestPath, item.Name), realPath)
			if err != nil {
				return err
			}
			return nil
		}()
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Explorer) Delete(http *gin.Context) {
	type ParamsValidate struct {
		Name     string   `json:"name" binding:"required"`
		FileList []string `json:"fileList" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	for _, path := range params.FileList {
		if path == "/" ||
			path == "./" ||
			path == "." ||
			strings.Contains(path, "*") {
			self.JsonResponseWithError(http, function.ErrorMessage(".containerExplorerEditDeleteUnsafe"), 500)
			return
		}
	}

	sshClient, afs, err := self.getClient(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		sshClient.Close()
	}()

	for _, item := range params.FileList {
		err := afs.Remove(item)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Explorer) ImportFileContent(http *gin.Context) {
	type ParamsValidate struct {
		Name     string `json:"name" binding:"required"`
		File     string `json:"file" binding:"required"`
		Content  string `json:"content"`
		DestPath string `json:"destPath" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if strings.HasPrefix(params.File, "/") {
		self.JsonResponseWithError(http, function.ErrorMessage(".containerExplorerInvalidFilename"), 500)
		return
	}

	sshClient, afs, err := self.getClient(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		sshClient.Close()
	}()

	file, err := afs.OpenFile(filepath.Join(params.DestPath, params.File), os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0o644)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	defer func() {
		_ = file.Close()
	}()
	_, err = file.WriteString(params.Content)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
}

func (self Explorer) MkDir(http *gin.Context) {
	type ParamsValidate struct {
		Name     string `json:"name" binding:"required"`
		DestPath string `json:"destPath" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	sshClient, afs, err := self.getClient(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		sshClient.Close()
	}()

	err = afs.MkdirAll(params.DestPath, os.ModePerm)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
}

func (self Explorer) Chmod(http *gin.Context) {
	type ParamsValidate struct {
		Name        string   `json:"name" binding:"required"`
		FileList    []string `json:"fileList" binding:"required"`
		Mod         int      `json:"mod" binding:"required"`
		HasChildren bool     `json:"hasChildren"`
		Owner       string   `json:"owner"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	sshClient, afs, err := self.getClient(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		sshClient.Close()
	}()

	for _, path := range params.FileList {
		err = afs.Chmod(path, os.FileMode(params.Mod))
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}

		if params.Owner != "" {
			//afs.Chown(path, params.Owner, params.Owner)
		}
	}

	self.JsonSuccessResponse(http)
	return
}

func (self Explorer) getClient(name string) (*ssh.Client, *afero.Afero, error) {
	dockerEnv, err := logic2.DockerEnv{}.GetEnvByName(name)
	if err != nil {
		return nil, nil, err
	}
	if !dockerEnv.EnableSSH || dockerEnv.SshServerInfo == nil {
		return nil, nil, function.ErrorMessage(".commonDataNotFoundOrDeleted")
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
