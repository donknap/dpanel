package controller

import (
	"errors"
	"fmt"
	"mime"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/agent/factor"
	archiveservice "github.com/donknap/dpanel/common/service/archive"
	"github.com/donknap/dpanel/common/service/docker"
	serviceafs "github.com/donknap/dpanel/common/service/fs/afs"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type Explorer struct {
	controller.Abstract
}

func (self Explorer) SyncDPanel(http *gin.Context) {
	type ParamsValidate struct {
		ComposeName string `json:"composeName" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	dockerSdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if dockerSdk == nil || dockerSdk.Client == nil || dockerSdk.Ctx == nil || dockerSdk.DockerEnv == nil {
		self.JsonResponseWithError(http, errors.New("docker client is required for dpanel sync"), 500)
		return
	}
	if dockerSdk.Name == define.DockerDefaultClientName {
		self.JsonSuccessResponse(http)
		return
	}
	dockerEnvName := define.DockerDefaultClientName
	if dockerSdk.DockerEnv.EnableComposePath {
		dockerEnvName = dockerSdk.Name
	}
	sourceDir := storage.Local{}.GetComposeProjectPath(dockerEnvName, params.ComposeName)
	relativeDir, err := filepath.Rel(storage.Local{}.GetStorageLocalPath(), sourceDir)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	fileSystem, err := (logic.Explorer{}).Afs(http,
		logic.ExplorerMountPoint(logic.ExplorerMountTypeContainer+":"+factor.ExplorerName), dockerSdk,
	)
	if err == nil {
		err = fileSystem.Import([]serviceafs.TransferFile{{
			Source: sourceDir,
			Target: path.Join("/", filepath.ToSlash(relativeDir)),
		}})
	}
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
}

func (self Explorer) Export(http *gin.Context) {
	type ParamsValidate struct {
		MountPoint        logic.ExplorerMountPoint `json:"mountPoint" binding:"required"`
		FileList          []string                 `json:"fileList" binding:"required"`
		ExportToPanelPath bool                     `json:"enableExportToPath"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	for _, filePath := range params.FileList {
		if filePath == "/" {
			self.JsonResponseWithError(http, errors.New("cannot export filesystem root"), 500)
			return
		}
	}
	dockerSdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	fileSystem, err := (logic.Explorer{}).Afs(http, params.MountPoint, dockerSdk)
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
	http.Header("Content-Type", download.ContentType)
	http.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": download.FileName}))
	http.File(download.Name())
}

func (self Explorer) ImportFileContent(http *gin.Context) {
	type ParamsValidate struct {
		MountPoint logic.ExplorerMountPoint `json:"mountPoint" binding:"required"`
		File       string                   `json:"file" binding:"required"`
		Content    string                   `json:"content"`
		DstPath    string                   `json:"dstPath" binding:"required"`
		FileMode   uint32                   `json:"fileMode"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if params.FileMode > 0o777 {
		self.JsonResponseWithError(http, errors.New("invalid file content import parameters"), 500)
		return
	}
	dockerSdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	fileSystem, err := (logic.Explorer{}).Afs(http, params.MountPoint, dockerSdk)
	if err == nil {
		err = (logic.Explorer{}).ImportFileContent(
			fileSystem, function.SafeFileName(params.File), params.Content, params.DstPath, params.FileMode,
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
		MountPoint logic.ExplorerMountPoint `json:"mountPoint" binding:"required"`
		FileList   []fileListItem           `json:"fileList" binding:"required"`
		DstPath    string                   `json:"dstPath" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	files := make([]logic.ExplorerImportFile, 0, len(params.FileList))
	for _, item := range params.FileList {
		name := strings.TrimLeft(function.SafePath(strings.ReplaceAll(item.Name, "\\", "/")), "/")
		if name == "" || name == "." {
			name = "file"
		}
		files = append(files, logic.ExplorerImportFile{Name: name, Path: item.Path})
	}
	dockerSdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	fileSystem, err := (logic.Explorer{}).Afs(http, params.MountPoint, dockerSdk)
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
		MountPoint logic.ExplorerMountPoint `json:"mountPoint" binding:"required"`
		File       []string                 `json:"file" binding:"required"`
		Path       string                   `json:"path" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	dockerSdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	fileSystem, err := (logic.Explorer{}).Afs(http, params.MountPoint, dockerSdk)
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
		MountPoint logic.ExplorerMountPoint `json:"mountPoint" binding:"required"`
		FileList   []string                 `json:"fileList" binding:"required"`
		Target     string                   `json:"target" binding:"required"`
		Format     archiveservice.Format    `json:"format" binding:"required"`
		Overwrite  bool                     `json:"overwrite"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if params.Target == "/" {
		self.JsonResponseWithError(http, errors.New("cannot create archive at filesystem root"), 500)
		return
	}
	switch params.Format {
	case archiveservice.FormatZip, archiveservice.FormatTar, archiveservice.FormatTarGz:
	default:
		self.JsonResponseWithError(http, archiveservice.ErrUnsupportedFormat, 500)
		return
	}
	for _, source := range params.FileList {
		if source == "/" {
			self.JsonResponseWithError(http, errors.New("cannot archive filesystem root"), 500)
			return
		}
		if source == params.Target {
			self.JsonResponseWithError(http, errors.New("archive target cannot be one of its sources"), 500)
			return
		}
	}
	dockerSdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	fileSystem, err := (logic.Explorer{}).Afs(http, params.MountPoint, dockerSdk)
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
		MountPoint logic.ExplorerMountPoint `json:"mountPoint" binding:"required"`
		FileList   []string                 `json:"fileList" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	for _, filePath := range params.FileList {
		if filePath == "" || filePath == "/" || strings.Contains(filePath, "*") {
			self.JsonResponseWithError(http, errors.New("unsafe delete path"), 500)
			return
		}
	}
	dockerSdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	fileSystem, err := (logic.Explorer{}).Afs(http, params.MountPoint, dockerSdk)
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
		MountPoint logic.ExplorerMountPoint `json:"mountPoint" binding:"required"`
		Path       string                   `json:"path"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	dockerSdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	fileSystem, err := (logic.Explorer{}).Afs(http, params.MountPoint, dockerSdk)
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
		MountPoint logic.ExplorerMountPoint `json:"mountPoint" binding:"required"`
		Path       string                   `json:"path" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	dockerSdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	fileSystem, err := (logic.Explorer{}).Afs(http, params.MountPoint, dockerSdk)
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
		MountPoint logic.ExplorerMountPoint `json:"mountPoint" binding:"required"`
		File       string                   `json:"file" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	dockerSdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	fileSystem, err := (logic.Explorer{}).Afs(http, params.MountPoint, dockerSdk)
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
		MountPoint  logic.ExplorerMountPoint `json:"mountPoint" binding:"required"`
		FileList    []string                 `json:"fileList" binding:"required"`
		Mod         string                   `json:"mod"`
		UID         *int                     `json:"uid"`
		GID         *int                     `json:"gid"`
		HasChildren bool                     `json:"hasChildren"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	for _, filePath := range params.FileList {
		if filePath == "" {
			self.JsonResponseWithError(http, errors.New("unsafe permission path"), 500)
			return
		}
		if params.HasChildren && filePath == "/" {
			self.JsonResponseWithError(http, errors.New("cannot recursively change filesystem root permissions"), 500)
			return
		}
	}
	dockerSdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	fileSystem, err := (logic.Explorer{}).Afs(http, params.MountPoint, dockerSdk)
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
		MountPoint logic.ExplorerMountPoint `json:"mountPoint" binding:"required"`
		Path       string                   `json:"path" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	dockerSdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	fileSystem, err := (logic.Explorer{}).Afs(http, params.MountPoint, dockerSdk)
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
		MountPoint logic.ExplorerMountPoint `json:"mountPoint" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	dockerSdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	fileSystem, err := (logic.Explorer{}).Afs(http, params.MountPoint, dockerSdk)
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
		MountPoint logic.ExplorerMountPoint `json:"mountPoint" binding:"required"`
		DstPath    string                   `json:"dstPath" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if params.DstPath == "/" {
		self.JsonResponseWithError(http, errors.New("cannot create filesystem root"), 500)
		return
	}
	dockerSdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	fileSystem, err := (logic.Explorer{}).Afs(http, params.MountPoint, dockerSdk)
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
		MountPoint logic.ExplorerMountPoint `json:"mountPoint" binding:"required"`
		SourceFile string                   `json:"sourceFile" binding:"required"`
		TargetFile string                   `json:"targetFile" binding:"required"`
		IsMove     bool                     `json:"isMove"`
		Overwrite  bool                     `json:"overwrite"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	dockerSdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	fileSystem, err := (logic.Explorer{}).Afs(http, params.MountPoint, dockerSdk)
	if err == nil {
		target := params.TargetFile
		if !path.IsAbs(target) {
			target = path.Join(path.Dir(params.SourceFile), function.SafePath(strings.ReplaceAll(target, "\\", "/")))
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
		if dockerSdk == nil || dockerSdk.Client == nil {
			err = errors.New("docker client is required to destroy explorer proxy")
		} else {
			lock := storage.NewMutex(fmt.Sprintf(storage.CacheKeyExplorerAfsLock, dockerSdk.Name, factor.ExplorerName))
			lock.Lock()
			err = factor.Destroy(dockerSdk, factor.ExplorerName)
			lock.Unlock()
		}
	}
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
}
