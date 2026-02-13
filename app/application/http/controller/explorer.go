package controller

import (
	"archive/tar"
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/compose"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/imports"
	"github.com/donknap/dpanel/common/service/fs"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/plugin"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	fs2 "github.com/donknap/dpanel/common/types/fs"
	"github.com/gin-gonic/gin"
	"github.com/h2non/filetype"
	"github.com/h2non/filetype/matchers"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type Explorer struct {
	controller.Abstract
}

func (self Explorer) Export(http *gin.Context) {
	type ParamsValidate struct {
		Name               string   `json:"name" binding:"required"`
		FileList           []string `json:"fileList" binding:"required"`
		EnableExportToPath bool     `json:"enableExportToPath"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	var err error

	containerInfo, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	fileName := strings.Trim(containerInfo.Name, "/") + "-" + time.Now().Format(define.DateYmdHis) + ".zip"
	tempFile, err := storage.Local{}.CreateSaveFile("export/file/" + fileName)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		_ = tempFile.Close()
		if !params.EnableExportToPath {
			_ = os.Remove(tempFile.Name())
		}
	}()

	pathInfo := make([]container.PathStat, 0)
	zipWriter := zip.NewWriter(tempFile)
	// 需要先将每个目录导出，然后再合并起来。直接导出整个容器效率太低
	for _, path := range params.FileList {
		out, info, err := docker.Sdk.Client.CopyFromContainer(docker.Sdk.Ctx, params.Name, path)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		pathInfo = append(pathInfo, info)

		tarReader := tar.NewReader(out)
		for {
			file, err := tarReader.Next()
			if err != nil {
				break
			}
			switch file.Typeflag {
			case tar.TypeReg, tar.TypeRegA, tar.TypeDir, tar.TypeGNUSparse:
				zipHeader := &zip.FileHeader{
					Name:               file.Name,
					Method:             zip.Deflate,
					UncompressedSize64: uint64(file.Size),
					Modified:           file.ModTime,
				}
				writer, _ := zipWriter.CreateHeader(zipHeader)
				_, _ = io.Copy(writer, tarReader)
			}
		}
		_ = out.Close()
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

	if params.EnableExportToPath {
		_ = notice.Message{}.Info(define.InfoMessageCommonExportInPath, "path", tempFile.Name())
		self.JsonSuccessResponse(http)
		return
	} else {
		http.Header("Content-Type", "application/zip")
		http.Header("Content-Disposition", "attachment; filename=export.zip")
		http.File(tempFile.Name())
		return
	}
}

func (self Explorer) ImportFileContent(http *gin.Context) {
	type ParamsValidate struct {
		File     string `json:"file" binding:"required"`
		Content  string `json:"content"`
		Name     string `json:"name" binding:"required"`
		DestPath string `json:"destPath" binding:"required"`
		FileMode int    `json:"fileMode"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	params.File = function.PathClean(params.File)
	params.DestPath = function.PathClean(params.DestPath)

	if strings.HasPrefix(params.File, "/") {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageContainerExplorerInvalidFilename), 500)
		return
	}
	fileMode := os.FileMode(params.FileMode)
	if params.FileMode == 0 {
		if pathStat, err := docker.Sdk.Client.ContainerStatPath(docker.Sdk.Ctx, params.Name, params.File); err == nil {
			fileMode = pathStat.Mode.Perm()
		}
	}
	if fileMode == 0 {
		fileMode = os.FileMode(0666)
	}
	importFile, err := imports.NewFileImport(params.DestPath, imports.WithImportContent(params.File, []byte(params.Content), fileMode))
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	err = docker.Sdk.ContainerImport(docker.Sdk.Ctx, params.Name, importFile)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
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
			realPath := storage.Local{}.GetSaveRealPath(s.Path)
			_ = os.Remove(realPath)
		}
	}()
	_, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	var options []imports.ImportFileOption
	for _, item := range params.FileList {
		realPath := storage.Local{}.GetSaveRealPath(item.Path)
		options = append(options, imports.WithImportFilePath(realPath, item.Name))
	}
	importFile, err := imports.NewFileImport(params.DestPath, options...)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	err = docker.Sdk.ContainerImport(docker.Sdk.Ctx, params.Name, importFile)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
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
	var options []imports.ImportFileOption
	for _, path := range params.File {
		targetFile, _ := storage.Local{}.CreateTempFile("")
		defer func() {
			_ = targetFile.Close()
			_ = os.Remove(targetFile.Name())
		}()
		_, err := docker.Sdk.ContainerReadFile(docker.Sdk.Ctx, params.Name, path, targetFile)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		fileType, err := filetype.MatchFile(targetFile.Name())
		switch fileType {
		case matchers.TypeZip:
			options = append(options, imports.WithImportZipFile(targetFile.Name()))
			break
		case matchers.TypeTar:
			options = append(options, imports.WithImportTarFile(targetFile.Name()))
			break
		case matchers.TypeGz:
			options = append(options, imports.WithImportTarGzFile(targetFile.Name()))
			break
		default:
			slog.Debug("explorer unzip ", "filetype", fileType, "err", err)
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageContainerExplorerUnzipTargetUnsupportedType), 500)
			return
		}
	}
	importFile, err := imports.NewFileImport(params.Path, options...)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	err = docker.Sdk.ContainerImport(docker.Sdk.Ctx, params.Name, importFile)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
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
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageContainerExplorerEditDeleteUnsafe), 500)
			return
		}
	}
	afs, err := fs.NewContainerExplorer(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	for _, path := range params.FileList {
		err = afs.RemoveAll(path)
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
	containerInfo, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if params.Path == "" && containerInfo.Config != nil {
		params.Path = containerInfo.Config.WorkingDir
	}
	if params.Path == "" {
		params.Path = "/"
	}
	afs, err := fs.NewContainerExplorer(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	list, err := afs.ReadDir(params.Path)
	result := make([]*fs2.FileData, 0)
	for _, info := range list {
		result = append(result, info.Sys().(*fs2.FileData))
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

	var tempChangeFileList = make(map[string]container.FilesystemChange)
	changeFileList, err := docker.Sdk.Client.ContainerDiff(docker.Sdk.Ctx, params.Name)
	if !function.IsEmptyArray(changeFileList) {
		for _, change := range changeFileList {
			tempChangeFileList[change.Path] = change
		}
	}

	for _, item := range result {
		if tempChangeFileList != nil {
			if change, ok := tempChangeFileList[item.Path]; ok {
				switch int(change.Kind) {
				case 0:
					item.Change = fs2.ChangeModified
					break
				case 1:
					item.Change = fs2.ChangeAdd
					break
				case 2:
					item.Change = fs2.ChangeDeleted
					break
				}
			}
		}
		if !function.IsEmptyArray(containerInfo.Mounts) {
			for _, mount := range containerInfo.Mounts {
				if strings.HasPrefix(item.Path, mount.Destination) {
					item.Change = fs2.ChangeVolume
					break
				}
			}
		}
	}
	var rootDirs []string
	if containerInfo.Config != nil && containerInfo.Config.WorkingDir != "" {
		rootDirs = append(rootDirs, containerInfo.Config.WorkingDir)
	}
	if containerInfo.Mounts != nil {
		rootDirs = append(rootDirs, function.PluckArrayWalk(containerInfo.Mounts, func(item container.MountPoint) (string, bool) {
			if pathStat, err := docker.Sdk.Client.ContainerStatPath(docker.Sdk.Ctx, containerInfo.ID, item.Destination); err == nil && pathStat.Mode.IsDir() {
				return item.Destination, true
			}
			return "", false
		})...)
	}
	self.JsonResponseWithoutError(http, gin.H{
		"currentPath": params.Path,
		"list":        result,
		"rootDirs":    rootDirs,
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
	pathStat, err := docker.Sdk.Client.ContainerStatPath(docker.Sdk.Ctx, params.Name, params.File)
	if pathStat.Size >= 1024*1024 {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageContainerExplorerEditFileMaxSize), 500)
		return
	}
	tempFile, err := storage.Local{}.CreateTempFile("")
	if err != nil {
		slog.Error("explorer", "get content", err)
	}
	defer func() {
		_ = os.Remove(tempFile.Name())
	}()

	_, err = docker.Sdk.ContainerReadFile(docker.Sdk.Ctx, params.Name, params.File, tempFile)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	_, err = tempFile.Seek(0, io.SeekStart)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	content, err := io.ReadAll(tempFile)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	fileType, _ := filetype.MatchFile(tempFile.Name())
	if fileType == filetype.Unknown {
		fileStat, _ := tempFile.Stat()
		self.JsonResponseWithoutError(http, gin.H{
			"content":  string(content),
			"fileMode": fileStat.Mode().Perm(),
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
	afs, err := fs.NewContainerExplorer(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	mode, err := strconv.ParseUint(params.Mod, 10, 32)
	for _, path := range params.FileList {
		err = afs.Chmod(path, os.FileMode(mode))
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
	var err error
	pathStat := container.PathStat{}
	var target = params.Path
	// 循环查找当前目录的链接最终对象
	for i := 0; i < 10; i++ {
		pathStat, err = docker.Sdk.Client.ContainerStatPath(docker.Sdk.Ctx, params.Name, target)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		if pathStat.LinkTarget != "" {
			target = pathStat.LinkTarget
		} else {
			break
		}
	}

	self.JsonResponseWithoutError(http, gin.H{
		"info": gin.H{
			"isDir":  pathStat.Mode.IsDir(),
			"target": target,
			"name":   filepath.Base(target),
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

	afs, err := fs.NewContainerExplorer(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
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
	afs, err := fs.NewContainerExplorer(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
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
	if !filepath.IsAbs(params.TargetFile) {
		params.TargetFile = filepath.Join(filepath.Dir(params.SourceFile), params.TargetFile)
	}
	targetFile, _ := storage.Local{}.CreateTempFile("")
	defer func() {
		_ = targetFile.Close()
		_ = os.Remove(targetFile.Name())
	}()
	_, err := docker.Sdk.ContainerReadFile(docker.Sdk.Ctx, params.Name, params.SourceFile, targetFile)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	importFile, err := imports.NewFileImport(filepath.Dir(params.TargetFile), imports.WithImportFile(targetFile, filepath.Base(params.TargetFile)))
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	err = docker.Sdk.ContainerImport(docker.Sdk.Ctx, params.Name, importFile)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if params.IsMove {
		afs, err := fs.NewContainerExplorer(params.Name)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		_ = afs.RemoveAll(params.SourceFile)
	}
	self.JsonSuccessResponse(http)
}

func (self Explorer) AttachVolume(http *gin.Context) {
	type ParamsValidate struct {
		Name string `json:"name" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	_, err := docker.Sdk.Client.VolumeInspect(docker.Sdk.Ctx, params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	_ = notice.Message{}.Info(".volumeMountSomeVolume")
	path := fmt.Sprintf("/%s", function.Md5(params.Name))
	explorerPlugin, err := plugin.NewPlugin(plugin.PluginExplorer, map[string]*plugin.TemplateParser{
		plugin.PluginExplorer: {
			ExtService: compose.ExtService{
				External: compose.ExternalItem{
					Volumes: []string{
						fmt.Sprintf("%s:%s", params.Name, path),
					},
				},
			},
		},
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if explorerPlugin.Exists() {
		_ = explorerPlugin.Destroy()
	}
	pluginName, err := explorerPlugin.Run()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"containerName": pluginName,
		"path":          path,
	})
}

func (self Explorer) DestroyProxyContainer(http *gin.Context) {
	_ = notice.Message{}.Info(".volumeMountSomeVolume")
	explorerPlugin, err := plugin.NewPlugin(plugin.PluginExplorer, nil)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if explorerPlugin.Exists() {
		_ = explorerPlugin.Destroy()
	}
	self.JsonSuccessResponse(http)
	return
}
