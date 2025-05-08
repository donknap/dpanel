package controller

import (
	"archive/tar"
	"archive/zip"
	"encoding/json"
	"errors"
	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/explorer"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/gin-gonic/gin"
	"github.com/h2non/filetype"
	"github.com/h2non/filetype/matchers"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"io"
	"log/slog"
	"os"
	"path/filepath"
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
	http.Header("Content-Type", "application/zip")
	http.Header("Content-Disposition", "attachment; filename=export.zip")
	http.File(tempFile.Name())
	return
}

func (self Explorer) ImportFileContent(http *gin.Context) {
	type ParamsValidate struct {
		File     string `json:"file" binding:"required"`
		Content  string `json:"content"`
		Name     string `json:"name" binding:"required"`
		DestPath string `json:"destPath" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if !strings.HasPrefix(params.File, "/") || !strings.HasPrefix(params.DestPath, "/") {
		self.JsonResponseWithError(http, function.ErrorMessage(".containerExplorerUseAbsolutePath"), 500)
		return
	}

	importFile, err := docker.NewFileImport(params.DestPath, docker.WithImportContent(params.File, []byte(params.Content), 0666))
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
			realPath := storage.Local{}.GetRealPath(s.Path)
			_ = os.Remove(realPath)
		}
	}()
	_, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	options := make([]docker.ImportFileOption, 0)
	for _, item := range params.FileList {
		realPath := storage.Local{}.GetRealPath(item.Path)
		options = append(options, docker.WithImportFilePath(realPath, item.Name))
	}
	importFile, err := docker.NewFileImport(params.DestPath, options...)
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
	options := make([]docker.ImportFileOption, 0)
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
		fileType, _ := filetype.MatchFile(targetFile.Name())
		switch fileType {
		case matchers.TypeZip:
			options = append(options, docker.WithImportZipFile(targetFile.Name()))
			break
		case matchers.TypeTar:
			options = append(options, docker.WithImportTarFile(targetFile.Name()))
			break
		case matchers.TypeGz:
			options = append(options, docker.WithImportTarGzFile(targetFile.Name()))
			break
		default:
			self.JsonResponseWithError(http, function.ErrorMessage(".containerExplorerUnzipTargetUnsupportedType"), 500)
			return
		}
	}
	importFile, err := docker.NewFileImport(params.Path, options...)
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
			self.JsonResponseWithError(http, errors.New("只可以删除指定的文件或是目录"), 500)
			return
		}
	}
	builder, err := explorer.NewExplorer(explorer.WithProxyContainerRunner(), explorer.WithRootPathFromContainer(params.Name))
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	err = builder.DeleteFileList(params.FileList)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
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
	containerInfo, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	builder, err := explorer.NewExplorer(explorer.WithProxyContainerRunner(), explorer.WithRootPathFromContainer(params.Name))
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	result, err := builder.GetListByPath(params.Path)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if function.IsEmptyArray(result) {
		self.JsonResponseWithoutError(http, gin.H{
			"list": result,
		})
		return
	}
	var tempChangeFileList = make(map[string]container.FilesystemChange)
	changeFileList, err := docker.Sdk.Client.ContainerDiff(docker.Sdk.Ctx, params.Name)
	if !function.IsEmptyArray(changeFileList) {
		for _, change := range changeFileList {
			tempChangeFileList[change.Path] = change
		}
	}

	for _, item := range result {
		if tempChangeFileList != nil {
			if change, ok := tempChangeFileList[item.Name]; ok {
				switch int(change.Kind) {
				case 0:
					item.Change = explorer.FileItemChangeModified
					break
				case 1:
					item.Change = explorer.FileItemChangeAdd
					break
				case 2:
					item.Change = explorer.FileItemChangeDeleted
					break
				}
			}
		}
		if !function.IsEmptyArray(containerInfo.Mounts) {
			for _, mount := range containerInfo.Mounts {
				if strings.HasPrefix(item.Name, mount.Destination) {
					item.Change = explorer.FileItemChangeVolume
					break
				}
			}
		}
	}
	self.JsonResponseWithoutError(http, gin.H{
		"list": result,
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
		self.JsonResponseWithError(http, errors.New("超过1M的文件请通过导入&导出修改文件"), 500)
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
		self.JsonResponseWithoutError(http, gin.H{
			"content": string(content),
		})
		return
	} else {
		self.JsonResponseWithError(http, function.ErrorMessage(".containerExplorerContentUnsupportedType"), 500)
		return
	}
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
	builder, err := explorer.NewExplorer(explorer.WithProxyContainerRunner(), explorer.WithRootPathFromContainer(params.Name))
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	err = builder.Chmod(params.FileList, params.Mod, params.HasChildren)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if params.Owner != "" {
		err = builder.Chown(params.Name, params.FileList, params.Owner, params.HasChildren)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
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

	tempFile, err := storage.Local{}.CreateTempFile("")
	if err != nil {
		slog.Error("explorer", "get content", err)
	}
	defer func() {
		_ = os.Remove(tempFile.Name())
	}()

	var result []byte
	if _, err = docker.Sdk.ContainerReadFile(docker.Sdk.Ctx, params.Name, "/etc/passwd", tempFile); err == nil {
		_, err = tempFile.Seek(0, io.SeekStart)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		result, err = io.ReadAll(tempFile)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	self.JsonResponseWithoutError(http, gin.H{
		"content": string(result),
	})
	return
}
