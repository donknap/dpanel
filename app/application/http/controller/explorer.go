package controller

import (
	"archive/tar"
	"archive/zip"
	"errors"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/archive"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Explorer struct {
	controller.Abstract
}

func (self Explorer) Export(http *gin.Context) {
	type ParamsValidate struct {
		Md5      string   `json:"md5" binding:"required"`
		FileList []string `json:"fileList" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	out, err := docker.Sdk.Client.ContainerExport(docker.Sdk.Ctx, params.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer out.Close()
	zipTempFile, _ := os.CreateTemp("", "dpanel")
	defer os.Remove(zipTempFile.Name())
	zipWriter := zip.NewWriter(zipTempFile)
	tarReader := tar.NewReader(out)
	for {
		file, err := tarReader.Next()
		if err != nil {
			break
		}
		switch file.Typeflag {
		case tar.TypeReg, tar.TypeRegA, tar.TypeDir, tar.TypeGNUSparse:
			for _, rootPath := range params.FileList {
				if strings.HasPrefix("/"+file.Name, rootPath) {
					zipHeader := &zip.FileHeader{
						Name:               file.Name,
						Method:             zip.Deflate,
						UncompressedSize64: uint64(file.Size),
						Modified:           file.ModTime,
					}
					writer, _ := zipWriter.CreateHeader(zipHeader)
					io.Copy(writer, tarReader)
				}
			}
		}
	}
	zipWriter.Close()
	http.Header("Content-Type", "application/zip")
	http.Header("Content-Disposition", "attachment; filename=export.zip")
	http.File(zipTempFile.Name())
	return
}

func (self Explorer) Import(http *gin.Context) {
	type ParamsValidate struct {
		Md5      string `json:"md5" binding:"required"`
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
	_, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, params.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	uploadTempDir, _ := os.MkdirTemp("", "dpanel")
	defer os.RemoveAll(uploadTempDir)
	for _, item := range params.FileList {
		err = os.Rename(storage.Local{}.GetRealPath(item.Path), uploadTempDir+"/"+item.Name)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	tarReader, err := archive.Tar(uploadTempDir, archive.Uncompressed)
	err = docker.Sdk.Client.CopyToContainer(docker.Sdk.Ctx,
		params.Md5,
		params.DestPath,
		tarReader,
		types.CopyToContainerOptions{},
	)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Explorer) Unzip(http *gin.Context) {
	type ParamsValidate struct {
		Md5  string `json:"md5" binding:"required"`
		File string `json:"file" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	explorer, err := logic.NewExplorer(params.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	err = explorer.Unzip(filepath.Dir(params.File), filepath.Base(params.File))
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Explorer) Delete(http *gin.Context) {
	type ParamsValidate struct {
		Md5      string   `json:"md5" binding:"required"`
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
	explorer, err := logic.NewExplorer(params.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	err = explorer.DeleteFileList(params.FileList)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Explorer) GetPathList(http *gin.Context) {
	type ParamsValidate struct {
		Md5  string `json:"md5" binding:"required"`
		Path string `json:"path" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	containerInfo, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, params.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	explorer, err := logic.NewExplorer(params.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	result, err := explorer.GetListByPath(params.Path)
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
	changeFileList, err := docker.Sdk.Client.ContainerDiff(docker.Sdk.Ctx, params.Md5)
	if !function.IsEmptyArray(changeFileList) {
		for _, change := range changeFileList {
			tempChangeFileList[change.Path] = change
		}
	}
	for _, item := range result {
		if tempChangeFileList != nil {
			if change, ok := tempChangeFileList[item.Name]; ok {
				item.Change = int(change.Kind)
			}
		}
		if !function.IsEmptyArray(containerInfo.Mounts) {
			for _, mount := range containerInfo.Mounts {
				if strings.HasPrefix(item.Name, mount.Destination) {
					item.Change = 100
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

func (self Explorer) Create(http *gin.Context) {
	type ParamsValidate struct {
		Md5      string `json:"md5" binding:"required"`
		DestPath string `json:"destPath" binding:"required"`
		IsDir    bool   `json:"isDir"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	explorer, err := logic.NewExplorer(params.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	err = explorer.Create(params.DestPath, params.IsDir)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Explorer) Rename(http *gin.Context) {
	type ParamsValidate struct {
		Md5     string `json:"md5" binding:"required"`
		File    string `json:"file" binding:"required"`
		NewName string `json:"newName" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	explorer, err := logic.NewExplorer(params.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	err = explorer.Rename(params.File, params.NewName)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}
