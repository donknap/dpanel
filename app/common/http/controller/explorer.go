package controller

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker/imports"
	"github.com/donknap/dpanel/common/service/exec/remote"
	"github.com/donknap/dpanel/common/service/fs"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	fsType "github.com/donknap/dpanel/common/types/fs"
	"github.com/gin-gonic/gin"
	"github.com/h2non/filetype"
	"github.com/pkg/sftp"
	"github.com/spf13/afero"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
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

	sshClient, afs, err := fs.NewSshExplorer(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		if sshClient != nil {
			sshClient.Close()
		}
	}()

	pathInfo := make([]container.PathStat, 0)
	zipWriter := zip.NewWriter(tempFile)

	for _, p := range params.FileList {
		p = function.Path2SystemSafe(p)
		err = func() error {
			file, err := afs.Open(p)
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
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageContainerExplorerInvalidFilename), 500)
		return
	}

	params.File = function.PathClean(params.File)
	params.DestPath = function.Path2SystemSafe(params.DestPath)

	sshClient, afs, err := fs.NewSshExplorer(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		if sshClient != nil {
			sshClient.Close()
		}
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

func (self Explorer) Import(http *gin.Context) {
	type fileListItem struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	type ParamsValidate struct {
		Name     string         `json:"name" binding:"required"`
		FileList []fileListItem `json:"fileList" binding:"required"`
		DestPath string         `json:"destPath" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	params.DestPath = function.Path2SystemSafe(params.DestPath)
	sshClient, afs, err := fs.NewSshExplorer(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		if sshClient != nil {
			sshClient.Close()
		}
	}()

	for _, item := range params.FileList {
		err = func() error {
			item.Path = function.Path2SystemSafe(item.Path)
			realPath, err := os.Open(storage.Local{}.GetSaveRealPath(item.Path))
			if err != nil {
				return err
			}
			defer func() {
				_ = realPath.Close()
				_ = os.Remove(realPath.Name())
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

func (self Explorer) Unzip(http *gin.Context) {
	type ParamsValidate struct {
		Name string   `json:"name" binding:"required"`
		File []string `json:"file" binding:"required"`
		Path string   `json:"path" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	sshClient, afs, err := fs.NewSshExplorer(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		if sshClient != nil {
			sshClient.Close()
		}
	}()
	options := make([]imports.ImportFileOption, 0)
	for _, p := range params.File {
		p = function.Path2SystemSafe(p)
		file, err := afs.OpenFile(p, os.O_RDONLY, 0o644)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		defer func() {
			file.Close()
		}()
		switch filepath.Ext(file.Name()) {
		case ".zip":
			fileInfo, _ := file.Stat()
			zipReader, err := zip.NewReader(file, fileInfo.Size())
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
			options = append(options, imports.WithImportZip(zipReader))
			break
		case ".tar":
			tarReader := tar.NewReader(file)
			options = append(options, imports.WithImportTar(tarReader))
			break
		case ".gz":
			gzReader, err := gzip.NewReader(file)
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
			tarReader := tar.NewReader(gzReader)
			options = append(options, imports.WithImportTar(tarReader))
			break
		default:
			slog.Debug("explorer unzip ", "filetype", filepath.Ext(file.Name()))
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageContainerExplorerUnzipTargetUnsupportedType), 500)
			return
		}
	}
	params.Path = function.Path2SystemSafe(params.Path)
	importFile, err := imports.NewFileImport(params.Path, options...)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	tarReader := importFile.TarReader()
	for {
		header, err := tarReader.Next()
		if err != nil {
			break
		}
		if header.FileInfo().IsDir() {
			err = afs.MkdirAll(header.Name, os.ModePerm)
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
			continue
		}
		err = afs.WriteReader(header.Name, tarReader)
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

	safePath := make([]string, 0)
	for _, p := range params.FileList {
		p = function.Path2SystemSafe(p)
		if p == "/" ||
			p == "./" ||
			p == "." ||
			strings.Contains(p, "*") {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageContainerExplorerEditDeleteUnsafe), 500)
			return
		}
		safePath = append(safePath, p)
	}

	sshClient, afs, err := fs.NewSshExplorer(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		if sshClient != nil {
			sshClient.Close()
		}
	}()

	for _, p := range safePath {
		err = self.deleteAll(afs, p)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Explorer) GetPathList(http *gin.Context) {
	type ParamsValidate struct {
		Name string `json:"name" binding:"required"`
		Path string `json:"path"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	params.Path = function.Path2SystemSafe(params.Path)
	sshClient, afs, err := fs.NewSshExplorer(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		if sshClient != nil {
			sshClient.Close()
		}
	}()
	if params.Path == "" {
		params.Path = "/"
		if sshClient != nil {
			if defaultPath, err := remote.QuickRun(sshClient, "pwd"); err == nil {
				params.Path = string(defaultPath)
			}
		} else {
			if v, err := os.UserHomeDir(); err == nil {
				params.Path = v
			}
		}
	}
	list, err := afs.ReadDir(params.Path)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	result := make([]*fsType.FileData, 0)
	for _, item := range list {
		fileData := &fsType.FileData{
			Path:     path.Join(params.Path, item.Name()),
			Name:     item.Name(),
			Mod:      item.Mode(),
			ModStr:   item.Mode().String(),
			ModTime:  item.ModTime(),
			Change:   fsType.ChangeDefault,
			Size:     item.Size(),
			User:     "",
			Group:    "",
			LinkName: "",
			IsDir:    item.Mode().IsDir(),
		}
		if v, ok := item.Sys().(*sftp.FileStat); ok {
			cacheKey := fmt.Sprintf(storage.CacheKeyExplorerUsername, params.Name, v.UID)
			if username, ok := storage.Cache.Get(cacheKey); ok {
				fileData.User = username.(string)
			}
			if fileData.User == "" && sshClient != nil {
				if username, err := remote.QuickRun(sshClient, fmt.Sprintf("id -un %d", v.UID)); err == nil {
					fileData.User = strings.TrimSpace(string(username))
					_ = storage.Cache.Add(cacheKey, string(username), time.Hour)
				} else {
					slog.Debug("explorer GetPathList username", "err", err)
					fileData.User = strconv.Itoa(int(v.UID))
				}
			}
			fileData.Group = strconv.Itoa(int(v.GID))
		}
		if fileData.CheckIsSymlink() {
			fileData.IsSymlink = true
			if sshClient != nil {
				if linkName, err := sshClient.SftpConn.ReadLink(path.Join(params.Path, item.Name())); err == nil {
					fileData.LinkName = linkName
				}
			} else {
				if linkName, err := os.Readlink(path.Join(params.Path, item.Name())); err == nil {
					fileData.LinkName = linkName
				}
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
	currentPath := params.Path
	// 如果当前是 windows 系统也转换成 linux 类的目录返回
	if v, ok := function.PathConvertWinPath2Unix(params.Path); ok {
		currentPath = v
	}
	self.JsonResponseWithoutError(http, gin.H{
		"currentPath": currentPath,
		"list":        result,
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
	sshClient, afs, err := fs.NewSshExplorer(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		if sshClient != nil {
			sshClient.Close()
		}
	}()
	params.File = function.Path2SystemSafe(params.File)
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
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageContainerExplorerEditFileMaxSize), 500)
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
			"content":  string(content),
			"fileMode": fileInfo.Mode().String(),
		})
		return
	} else {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageContainerExplorerContentUnsupportedType), 500)
		return
	}
}

func (self Explorer) Chmod(http *gin.Context) {
	type ParamsValidate struct {
		Name        string   `json:"name" binding:"required"`
		FileList    []string `json:"fileList" binding:"required"`
		Mod         string   `json:"mod" binding:"required"`
		User        string   `json:"user"`
		Group       string   `json:"group"`
		HasChildren bool     `json:"hasChildren"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	sshClient, afs, err := fs.NewSshExplorer(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		if sshClient != nil {
			sshClient.Close()
		}
	}()
	mode, err := strconv.ParseUint(params.Mod, 8, 32)
	for _, p := range params.FileList {
		p = function.Path2SystemSafe(p)
		err = afs.Chmod(p, os.FileMode(mode))
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}

		if params.User != "" && params.Group != "" {
			//afs.Chown(path, params.User, params.Group)
		}
	}

	self.JsonSuccessResponse(http)
	return
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
	sshClient, afs, err := fs.NewSshExplorer(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		if sshClient != nil {
			sshClient.Close()
		}
	}()
	params.Path = function.Path2SystemSafe(params.Path)
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

func (self Explorer) GetUserList(http *gin.Context) {
	type ParamsValidate struct {
		Name string `json:"name" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	sshClient, afs, err := fs.NewSshExplorer(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		if sshClient != nil {
			sshClient.Close()
		}
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

func (self Explorer) MkDir(http *gin.Context) {
	type ParamsValidate struct {
		Name     string `json:"name" binding:"required"`
		DestPath string `json:"destPath" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	sshClient, afs, err := fs.NewSshExplorer(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		if sshClient != nil {
			sshClient.Close()
		}
	}()
	params.DestPath = function.Path2SystemSafe(params.DestPath)
	err = afs.MkdirAll(params.DestPath, os.ModePerm)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
}

func (self Explorer) Copy(http *gin.Context) {
	type ParamsValidate struct {
		Name       string `json:"name" binding:"required"`
		SourceFile string `json:"sourceFile" binding:"required"`
		TargetFile string `json:"targetFile" binding:"required"`
		IsMove     bool   `json:"isMove"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	params.SourceFile = function.Path2SystemSafe(params.SourceFile)
	params.TargetFile = function.Path2SystemSafe(params.TargetFile)
	if !filepath.IsAbs(params.TargetFile) {
		params.TargetFile = filepath.Join(filepath.Dir(params.SourceFile), params.TargetFile)
	}

	sshClient, afs, err := fs.NewSshExplorer(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		if sshClient != nil {
			sshClient.Close()
		}
	}()

	if ok, _ := afs.Exists(params.TargetFile); ok {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonIdAlreadyExists, "name", filepath.Base(params.TargetFile)), 500)
		return
	}

	sourceFile, err := afs.Open(params.SourceFile)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		_ = sourceFile.Close()
	}()

	sourceFileStat, _ := sourceFile.Stat()
	if sourceFileStat.IsDir() {
		targetFileRoot := params.TargetFile
		err = afs.MkdirAll(targetFileRoot, sourceFileStat.Mode())
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		list, err := sourceFile.Readdir(-1)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		var errs []error
		waitGroup := sync.WaitGroup{}
		for _, info := range list {
			go func() {
				waitGroup.Add(1)
				defer func() {
					waitGroup.Done()
				}()
				newFileName := filepath.Base(info.Name())
				if info.IsDir() {
					err = afs.Mkdir(filepath.Join(targetFileRoot, newFileName), info.Mode())
					if err != nil {
						errs = append(errs, err)
						return
					}
				} else {
					tf, err := afs.OpenFile(filepath.Join(targetFileRoot, newFileName), os.O_CREATE|os.O_RDWR|os.O_TRUNC, info.Mode())
					if err != nil {
						errs = append(errs, err)
						return
					}
					defer func() {
						_ = tf.Close()
					}()
					sf, err := afs.Open(filepath.Join(params.SourceFile, info.Name()))
					if err != nil {
						errs = append(errs, err)
						return
					}
					defer func() {
						_ = sf.Close()
					}()
					_, err = io.Copy(tf, sf)
					if err != nil {
						errs = append(errs, err)
						return
					}
				}
			}()
		}
		waitGroup.Wait()
		if errs != nil {
			self.JsonResponseWithError(http, errors.Join(errs...), 500)
			return
		}
	} else {
		targetFile, err := afs.OpenFile(params.TargetFile, os.O_CREATE|os.O_RDWR, sourceFileStat.Mode())
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		_, err = io.Copy(targetFile, sourceFile)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	if params.IsMove {
		err = self.deleteAll(afs, params.SourceFile)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	self.JsonSuccessResponse(http)
}

func (self Explorer) deleteAll(afs *afero.Afero, path string) error {
	file, err := afs.Open(path)
	if err != nil {
		return err
	}
	fileInfo, _ := file.Stat()
	// 删除文件之前需要先关闭，否则 windows 会报错文件被占用
	_ = file.Close()
	if fileInfo.IsDir() {
		files, err := afs.ReadDir(path)
		if err != nil {
			return err
		}
		for _, item := range files {
			err = self.deleteAll(afs, filepath.Join(path, item.Name()))
			if err != nil {
				return err
			}
		}
	}
	return afs.Remove(path)
}
