package controller

import (
	"archive/tar"
	"archive/zip"
	"errors"
	"fmt"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var fileTree = make(map[string][]*fileItem)

type fileItem struct {
	ShowName string `json:"showName"`
	Name     string `json:"name"`
	Typeflag byte   `json:"typeFlag"`
	LinkName string `json:"linkName"`
	Size     int64  `json:"size"`
	Mode     int64  `json:"mode"`
	IsDir    bool   `json:"isDir"`
	ModTime  string `json:"modTime"`
	Change   int    `json:"change"`
}

type Tree struct {
	controller.Abstract
}

func (self Tree) GetList(http *gin.Context) {
	type ParamsValidate struct {
		Md5      string `json:"md5" binding:"required"`
		Path     string `json:"path" binding:"required"`
		InitData bool   `json:"initData"`
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

	if params.InitData {
		changeFileList, err := docker.Sdk.Client.ContainerDiff(docker.Sdk.Ctx, params.Md5)
		out, _, err := docker.Sdk.Client.CopyFromContainer(docker.Sdk.Ctx, params.Md5, params.Path)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		var fileList []*fileItem
		tarArchive := tar.NewReader(out)
		for {
			file, err := tarArchive.Next()
			if err != nil {
				break
			}
			fileRow := &fileItem{
				ShowName: filepath.Base(file.Name),
				Name:     file.Name,
				LinkName: file.Linkname,
				Typeflag: file.Typeflag,
				IsDir:    file.Typeflag == tar.TypeDir,
				Mode:     file.Mode,
				ModTime:  file.ModTime.Format(function.YmdHis),
				Size:     file.Size,
				Change:   -1,
			}
			if !function.IsEmptyArray(changeFileList) {
				for _, change := range changeFileList {
					if strings.TrimSuffix(file.Name, "/") == change.Path {
						fileRow.Change = int(change.Kind)
						break
					}
				}
			}
			if !function.IsEmptyArray(containerInfo.Mounts) {
				for _, mount := range containerInfo.Mounts {
					if strings.HasPrefix(file.Name, mount.Destination) {
						fileRow.Change = 100
						break
					}
				}
			}
			fileList = append(fileList, fileRow)
		}
		fileTree[params.Md5] = fileList
	}
	if function.IsEmptyArray(fileTree[params.Md5]) {
		self.JsonResponseWithError(http, errors.New("请先获取目录"), 500)
		return
	}
	var fileList []*fileItem
	path := strings.TrimSuffix(params.Path, "/") + "/"
	level := strings.Count(params.Path, "/")
	for _, item := range fileTree[params.Md5] {
		if strings.HasPrefix(item.Name, path) {
			pathName := strings.TrimSuffix(item.Name, "/")
			if strings.Count(pathName, "/") == level {
				fileList = append(fileList, item)
			}
		}
	}
	sort.SliceStable(fileList, func(i, j int) bool {
		return fileList[i].Typeflag > fileList[j].Typeflag
	})
	self.JsonResponseWithoutError(http, gin.H{
		"list": fileList,
	})
	return
}

func (self Tree) Export(http *gin.Context) {
	type ParamsValidate struct {
		Md5      string   `json:"md5" binding:"required"`
		FileList []string `json:"fileList" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	out, _, err := docker.Sdk.Client.CopyFromContainer(docker.Sdk.Ctx, params.Md5, "/")
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	zipTempFile, _ := os.CreateTemp("", "dpanel")
	zipWriter := zip.NewWriter(zipTempFile)
	defer zipWriter.Close()
	tarArchive := tar.NewReader(out)
	defer out.Close()

	fmt.Printf("%v \n", zipTempFile.Name())
	for {
		file, err := tarArchive.Next()
		if err != nil {
			break
		}
		for _, rootPath := range params.FileList {
			if strings.HasPrefix(file.Name, rootPath) {
				content := make([]byte, file.Size)
				tarArchive.Read(content)
				w, _ := zipWriter.Create(file.Name)
				w.Write(content)
				continue
			}
		}
	}
}
