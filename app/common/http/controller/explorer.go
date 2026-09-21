package controller

import (
	"errors"
	"fmt"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/donknap/dpanel/app/common/logic"
	archiveservice "github.com/donknap/dpanel/common/service/archive"
	"github.com/donknap/dpanel/common/service/docker"
	serviceafs "github.com/donknap/dpanel/common/service/fs/afs"
	"github.com/donknap/dpanel/common/service/plugin"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type Explorer struct {
	controller.Abstract
}

var explorerMountNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

func (self Explorer) afs(http *gin.Context, mountPointValue string) (serviceafs.Fs, error) {
	mountType, mountName, ok := strings.Cut(mountPointValue, ":")
	if !ok || mountType == "" || mountName == "" || strings.Contains(mountName, ":") {
		return nil, errors.New("invalid explorer mount point")
	}
	switch mountType {
	case logic.ExplorerMountTypeLocal:
		if mountName != logic.ExplorerMountHost && mountName != logic.ExplorerMountDPanel {
			return nil, errors.New("invalid local explorer mount point")
		}
	case logic.ExplorerMountTypeContainer, logic.ExplorerMountTypeVolume, logic.ExplorerMountTypeDocker:
		if !explorerMountNamePattern.MatchString(mountName) {
			return nil, errors.New("invalid explorer mount point name")
		}
	default:
		return nil, errors.New("unknown explorer mount point type")
	}
	var dockerSdk *docker.Client
	var err error
	if mountType == logic.ExplorerMountTypeContainer || mountType == logic.ExplorerMountTypeVolume {
		dockerSdk, err = docker.NewClientWithUser(http)
		if err != nil {
			return nil, err
		}
	}
	return (logic.Explorer{}).Afs(http, mountType, mountName, dockerSdk)
}

func (self Explorer) Export(http *gin.Context) {
	type ParamsValidate struct {
		MountPoint        string   `json:"mountPoint" binding:"required"`
		FileList          []string `json:"fileList" binding:"required"`
		ExportToPanelPath bool     `json:"enableExportToPath"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if err := validateExplorerPaths(params.FileList, false); err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	fileSystem, err := self.afs(http, params.MountPoint)
	var download *logic.ExplorerDownload
	if err == nil {
		download, err = (logic.Explorer{}).Export(fileSystem, params.FileList, params.ExportToPanelPath)
	}
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if download == nil {
		self.JsonSuccessResponse(http)
		return
	}
	defer download.Close()
	http.Header("Content-Type", "application/zip")
	http.Header("Content-Disposition", "attachment; filename=export.zip")
	http.File(download.Name())
}

func (self Explorer) ImportFileContent(http *gin.Context) {
	type ParamsValidate struct {
		MountPoint string `json:"mountPoint" binding:"required"`
		File       string `json:"file" binding:"required"`
		Content    string `json:"content"`
		DstPath    string `json:"dstPath" binding:"required"`
		FileMode   uint32 `json:"fileMode"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if !validExplorerFileName(params.File) || !validExplorerPath(params.DstPath, true) || params.FileMode > 0o777 {
		self.JsonResponseWithError(http, errors.New("invalid file content import parameters"), 500)
		return
	}
	fileSystem, err := self.afs(http, params.MountPoint)
	if err == nil {
		err = (logic.Explorer{}).ImportFileContent(
			fileSystem, params.File, params.Content, params.DstPath, params.FileMode,
		)
	}
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
		MountPoint string         `json:"mountPoint" binding:"required"`
		FileList   []fileListItem `json:"fileList" binding:"required"`
		DstPath    string         `json:"dstPath" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if !validExplorerPath(params.DstPath, true) {
		self.JsonResponseWithError(http, errors.New("invalid import destination"), 500)
		return
	}
	files := make([]logic.ExplorerImportFile, 0, len(params.FileList))
	for _, item := range params.FileList {
		if !validExplorerRelativePath(item.Name) {
			self.JsonResponseWithError(http, errors.New("invalid import file name"), 500)
			return
		}
		files = append(files, logic.ExplorerImportFile{Name: item.Name, Path: item.Path})
	}
	fileSystem, err := self.afs(http, params.MountPoint)
	if err == nil {
		err = (logic.Explorer{}).Import(fileSystem, params.DstPath, files)
	}
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
}

func (self Explorer) Unzip(http *gin.Context) {
	type ParamsValidate struct {
		MountPoint string   `json:"mountPoint" binding:"required"`
		File       []string `json:"file" binding:"required"`
		Path       string   `json:"path" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if !validExplorerPath(params.Path, true) {
		self.JsonResponseWithError(http, errors.New("invalid unzip destination"), 500)
		return
	}
	for _, filePath := range params.File {
		if !validExplorerPath(filePath, false) {
			self.JsonResponseWithError(http, errors.New("invalid archive path"), 500)
			return
		}
	}
	fileSystem, err := self.afs(http, params.MountPoint)
	if err == nil {
		err = (logic.Explorer{}).UnArchive(fileSystem, params.File, params.Path)
	}
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
}

func (self Explorer) Archive(http *gin.Context) {
	type ParamsValidate struct {
		MountPoint string                `json:"mountPoint" binding:"required"`
		FileList   []string              `json:"fileList" binding:"required"`
		Target     string                `json:"target" binding:"required"`
		Format     archiveservice.Format `json:"format" binding:"required"`
		Overwrite  bool                  `json:"overwrite"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if err := validateExplorerPaths(params.FileList, false); err != nil || !validExplorerPath(params.Target, false) {
		self.JsonResponseWithError(http, errors.New("invalid archive path"), 500)
		return
	}
	switch params.Format {
	case archiveservice.FormatZip, archiveservice.FormatTar, archiveservice.FormatTarGz:
	default:
		self.JsonResponseWithError(http, archiveservice.ErrUnsupportedFormat, 500)
		return
	}
	for _, source := range params.FileList {
		if source == params.Target {
			self.JsonResponseWithError(http, errors.New("archive target cannot be one of its sources"), 500)
			return
		}
	}
	fileSystem, err := self.afs(http, params.MountPoint)
	if err == nil {
		err = (logic.Explorer{}).Archive(fileSystem, params.FileList, params.Target, params.Format, params.Overwrite)
	}
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
}

func (self Explorer) Delete(http *gin.Context) {
	type ParamsValidate struct {
		MountPoint string   `json:"mountPoint" binding:"required"`
		FileList   []string `json:"fileList" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if err := validateExplorerPaths(params.FileList, false); err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	for _, filePath := range params.FileList {
		if strings.Contains(filePath, "*") {
			self.JsonResponseWithError(http, errors.New("unsafe delete path"), 500)
			return
		}
	}
	fileSystem, err := self.afs(http, params.MountPoint)
	if err == nil {
		for _, filePath := range params.FileList {
			if err = fileSystem.RemoveAll(filePath); err != nil {
				break
			}
		}
	}
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
}

func (self Explorer) GetPathList(http *gin.Context) {
	type ParamsValidate struct {
		MountPoint string `json:"mountPoint" binding:"required"`
		Path       string `json:"path"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if params.Path != "" && !validExplorerPath(params.Path, true) {
		self.JsonResponseWithError(http, errors.New("invalid explorer path"), 500)
		return
	}
	fileSystem, err := self.afs(http, params.MountPoint)
	currentPath := params.Path
	if err == nil && currentPath == "" {
		currentPath = fileSystem.WorkingDir()
	}
	var listErr error
	var list any
	if err == nil {
		fileList, readErr := fileSystem.List(currentPath)
		if readErr == nil {
			sort.Slice(fileList, func(i, j int) bool {
				if fileList[i].IsDir != fileList[j].IsDir {
					return fileList[i].IsDir
				}
				return fileList[i].Name < fileList[j].Name
			})
			list = fileList
		} else {
			listErr = readErr
		}
	}
	var rootDirs []string
	if err == nil && listErr == nil {
		rootDirs, listErr = fileSystem.RootDirs()
	}
	if err == nil && listErr == nil {
		self.JsonResponseWithoutError(http, gin.H{
			"currentPath": currentPath,
			"list":        list,
			"rootDirs":    rootDirs,
		})
		return
	}
	if err == nil {
		err = listErr
	}
	self.JsonResponseWithError(http, err, 500)
}

func (self Explorer) GetPathSize(http *gin.Context) {
	type ParamsValidate struct {
		MountPoint string `json:"mountPoint" binding:"required"`
		Path       string `json:"path" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if !validExplorerPath(params.Path, true) {
		self.JsonResponseWithError(http, errors.New("invalid path"), 500)
		return
	}
	fileSystem, err := self.afs(http, params.MountPoint)
	var size int64
	if err == nil {
		size, err = fileSystem.PathSize(params.Path)
	}
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{"size": size})
}

func (self Explorer) GetContent(http *gin.Context) {
	type ParamsValidate struct {
		MountPoint string `json:"mountPoint" binding:"required"`
		File       string `json:"file" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if !validExplorerPath(params.File, false) {
		self.JsonResponseWithError(http, errors.New("invalid file path"), 500)
		return
	}
	fileSystem, err := self.afs(http, params.MountPoint)
	var result logic.ExplorerContent
	if err == nil {
		result, err = (logic.Explorer{}).GetContent(fileSystem, params.File)
	}
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{"content": result.Content, "fileMode": result.FileMode})
}

func (self Explorer) Permission(http *gin.Context) {
	type ParamsValidate struct {
		MountPoint  string   `json:"mountPoint" binding:"required"`
		FileList    []string `json:"fileList" binding:"required"`
		Mod         string   `json:"mod"`
		UID         *int     `json:"uid"`
		GID         *int     `json:"gid"`
		HasChildren bool     `json:"hasChildren"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if err := validateExplorerPaths(params.FileList, !params.HasChildren); err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	fileSystem, err := self.afs(http, params.MountPoint)
	if err == nil {
		err = (logic.Explorer{}).Permission(
			fileSystem, params.FileList, params.Mod, params.UID, params.GID, params.HasChildren,
		)
	}
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
}

func (self Explorer) GetFileStat(http *gin.Context) {
	type ParamsValidate struct {
		MountPoint string `json:"mountPoint" binding:"required"`
		Path       string `json:"path" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if !validExplorerPath(params.Path, true) {
		self.JsonResponseWithError(http, errors.New("invalid file path"), 500)
		return
	}
	fileSystem, err := self.afs(http, params.MountPoint)
	if err == nil {
		result, statErr := fileSystem.Info(params.Path)
		if statErr == nil {
			self.JsonResponseWithoutError(http, gin.H{"info": gin.H{
				"isDir":  result.IsDir,
				"target": result.Path,
				"name":   result.Name,
			}})
			return
		}
		err = statErr
	}
	self.JsonResponseWithError(http, err, 500)
}

func (self Explorer) GetUserList(http *gin.Context) {
	type ParamsValidate struct {
		MountPoint string `json:"mountPoint" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	fileSystem, err := self.afs(http, params.MountPoint)
	if err == nil {
		identities, userErr := fileSystem.Users()
		if userErr == nil {
			self.JsonResponseWithoutError(http, identities)
			return
		}
		err = userErr
	}
	self.JsonResponseWithError(http, err, 500)
}

func (self Explorer) MkDir(http *gin.Context) {
	type ParamsValidate struct {
		MountPoint string `json:"mountPoint" binding:"required"`
		DstPath    string `json:"dstPath" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if !validExplorerPath(params.DstPath, false) {
		self.JsonResponseWithError(http, errors.New("invalid directory path"), 500)
		return
	}
	fileSystem, err := self.afs(http, params.MountPoint)
	if err == nil {
		err = fileSystem.MkdirAll(params.DstPath, os.ModePerm)
	}
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
}

func (self Explorer) Copy(http *gin.Context) {
	type ParamsValidate struct {
		MountPoint string `json:"mountPoint" binding:"required"`
		SourceFile string `json:"sourceFile" binding:"required"`
		TargetFile string `json:"targetFile" binding:"required"`
		IsMove     bool   `json:"isMove"`
		Overwrite  bool   `json:"overwrite"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if !validExplorerPath(params.SourceFile, false) || !validExplorerTransferTarget(params.TargetFile) {
		self.JsonResponseWithError(http, errors.New("invalid copy or move path"), 500)
		return
	}
	fileSystem, err := self.afs(http, params.MountPoint)
	if err == nil {
		target := params.TargetFile
		if !path.IsAbs(target) {
			target = path.Join(path.Dir(params.SourceFile), target)
		}
		if params.IsMove {
			err = fileSystem.Move(params.SourceFile, target, params.Overwrite)
		} else {
			err = fileSystem.Copy(params.SourceFile, target, params.Overwrite)
		}
	}
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
}

func (self Explorer) DestroyProxyContainer(http *gin.Context) {
	dockerSdk, err := docker.NewClientWithUser(http)
	if err == nil {
		var explorerPlugin *plugin.Plugin
		explorerPlugin, err = plugin.NewPlugin(dockerSdk, plugin.ExplorerName, plugin.CreateOption{Init: false})
		if err == nil {
			lock := storage.NewMutex(fmt.Sprintf(storage.CacheKeyExplorerAfsLock, dockerSdk.Name, plugin.ExplorerName))
			lock.Lock()
			defer lock.Unlock()
			storage.Cache.Delete(fmt.Sprintf(storage.CacheKeyExplorerAfs, dockerSdk.Name, plugin.ExplorerName))
			err = explorerPlugin.Close()
		}
	}
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
}

func validExplorerPath(value string, allowRoot bool) bool {
	return value != "" && strings.IndexByte(value, 0) < 0 && path.IsAbs(value) && path.Clean(value) == value && (allowRoot || value != "/")
}

func validExplorerFileName(value string) bool {
	return value != "" && value != "." && strings.IndexByte(value, 0) < 0 && !path.IsAbs(value) && path.Clean(value) == value && path.Base(value) == value
}

func validExplorerRelativePath(value string) bool {
	return value != "" && value != "." && strings.IndexByte(value, 0) < 0 && !path.IsAbs(value) && path.Clean(value) == value && value != ".." && !strings.HasPrefix(value, "../")
}

func validExplorerTransferTarget(value string) bool {
	if path.IsAbs(value) {
		return validExplorerPath(value, false)
	}
	return validExplorerRelativePath(value)
}

func validateExplorerPaths(values []string, allowRoot bool) error {
	for _, value := range values {
		if !validExplorerPath(value, allowRoot) {
			return errors.New("invalid explorer path")
		}
	}
	return nil
}
