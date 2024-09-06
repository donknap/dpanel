package controller

import (
	"errors"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"io"
	"os"
	"strings"
)

type Image struct {
	controller.Abstract
}

func (self Image) ImportByContainerTar(http *gin.Context) {
	type ParamsValidate struct {
		Tar      string   `json:"tar" binding:"required"`
		Tag      string   `json:"tag" binding:"required"`
		Registry string   `json:"registry"`
		Cmd      string   `json:"cmd" binding:"required"`
		WorkDir  string   `json:"workDir"`
		Expose   []string `json:"expose"`
		Env      []string `json:"env"`
		Volume   []string `json:"volume"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	imageName := logic.Image{}.GetImageName(&logic.ImageNameOption{
		Registry: params.Registry,
		Name:     params.Tag,
	})
	imageInfo, _, err := docker.Sdk.Client.ImageInspectWithRaw(docker.Sdk.Ctx, imageName)
	if imageInfo.ID != "" {
		self.JsonResponseWithError(http, errors.New("镜像名称已经存在"), 500)
		return
	}
	containerTar, err := os.Open(storage.Local{}.GetRealPath(params.Tar))
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	change := []string{
		"CMD " + params.Cmd,
		"WORKDIR " + params.WorkDir,
	}
	for _, port := range params.Expose {
		change = append(change, "EXPOSE "+port)
	}
	for _, env := range params.Env {
		change = append(change, "ENV "+env)
	}
	for _, volume := range params.Volume {
		change = append(change, "VOLUME "+volume)
	}
	out, err := docker.Sdk.Client.ImageImport(docker.Sdk.Ctx, image.ImportSource{
		Source:     containerTar,
		SourceName: "-",
	}, imageName, image.ImportOptions{
		Changes: change,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer out.Close()

	_, err = io.Copy(os.Stdout, out)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	_ = os.Remove(storage.Local{}.GetRealPath(params.Tar))

	self.JsonSuccessResponse(http)
	return
}

func (self Image) ImportByImageTar(http *gin.Context) {
	type ParamsValidate struct {
		Tar      string `json:"tar" binding:"required"`
		Tag      string `json:"tag" binding:"required"`
		Registry string `json:"registry"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	imageName := logic.Image{}.GetImageName(&logic.ImageNameOption{
		Registry: params.Registry,
		Name:     params.Tag,
	})
	imageInfo, _, err := docker.Sdk.Client.ImageInspectWithRaw(docker.Sdk.Ctx, imageName)
	if imageInfo.ID != "" {
		self.JsonResponseWithError(http, errors.New("镜像名称已经存在"), 500)
		return
	}
	imageTar, err := os.Open(storage.Local{}.GetRealPath(params.Tar))
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	out, err := docker.Sdk.Client.ImageLoad(docker.Sdk.Ctx, imageTar, false)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	progressChan := docker.Sdk.Progress(out.Body, imageName)
	for {
		select {
		case message, ok := <-progressChan:
			if !ok {
				if err != nil {
					self.JsonResponseWithError(http, message.Err, 500)
					return
				}
			}
			if message.Stream != nil && strings.Contains(message.Stream.Stream, "Loaded image:") {
				importImageName := strings.Split(message.Stream.Stream, "Loaded image:")[1]
				err = docker.Sdk.Client.ImageTag(docker.Sdk.Ctx, strings.TrimSpace(importImageName), imageName)
				if err != nil {
					self.JsonResponseWithError(http, err, 500)
					return
				}
				_ = os.Remove(storage.Local{}.GetRealPath(params.Tar))
				self.JsonSuccessResponse(http)
				return
			}
		}
	}
}

func (self Image) CreateByDockerfile(http *gin.Context) {
	type ParamsValidate struct {
		Id              int32  `json:"id"`
		Registry        string `json:"registry"`
		Tag             string `json:"tag" binding:"required"`
		Title           string `json:"title"`
		BuildType       string `json:"buildType" binding:"required"`
		BuildDockerfile string `json:"buildDockerfile" binding:"omitempty"`
		BuildGit        string `json:"buildGit" binding:"omitempty"`
		BuildZip        string `json:"buildZip" binding:"omitempty"`
		BuildRoot       string `json:"buildRoot" binding:"omitempty"`
		Platform        string `json:"platform"`
		PlatformArch    string `json:"platformArch"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if params.BuildDockerfile == "" && params.BuildZip == "" && params.BuildGit == "" {
		self.JsonResponseWithError(http, errors.New("至少需要指定 Dockerfile、Zip 包或是 Git 地址"), 500)
		return
	}
	if params.BuildZip != "" && params.BuildGit != "" {
		self.JsonResponseWithError(http, errors.New("zip 包和 git 地址只需要只定一项"), 500)
		return
	}

	imageName := logic.Image{}.GetImageName(&logic.ImageNameOption{
		Registry: params.Registry,
		Name:     params.Tag,
	})

	mustHasZipFile := false
	buildImageTask := &logic.BuildImageMessage{
		Tag: imageName,
	}

	if params.BuildRoot != "" {
		buildImageTask.Context = "./" + strings.Trim(strings.Trim(strings.Trim(params.BuildRoot, ""), "./"), "/")
	}
	addStr := []string{
		"ADD",
		"COPY",
	}
	for _, str := range addStr {
		if strings.HasPrefix(strings.ToUpper(params.BuildDockerfile), str) {
			mustHasZipFile = true
		}
	}
	if mustHasZipFile {
		if params.BuildZip == "" && params.BuildGit == "" {
			self.JsonResponseWithError(http, errors.New("Dockerfile中包含添加文件操作，请上传对应的Zip包或是指定Git仓库"), 500)
			return
		}
	}
	if params.BuildZip != "" {
		path := storage.Local{}.GetRealPath(params.BuildZip)
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			self.JsonResponseWithError(http, errors.New("请先上传压缩包"), 500)
			return
		}
		buildImageTask.ZipPath = path
	}
	if params.BuildDockerfile != "" {
		buildImageTask.DockerFileContent = []byte(params.BuildDockerfile)
	}
	if params.BuildGit != "" {
		buildImageTask.GitUrl = params.BuildGit
	}
	imageNew := &entity.Image{
		Tag:   imageName,
		Title: params.Title,
		Setting: &accessor.ImageSettingOption{
			Registry:        params.Registry,
			BuildGit:        params.BuildGit,
			BuildDockerfile: params.BuildDockerfile,
			BuildRoot:       params.BuildRoot,
			Platform:        params.Platform,
		},
		BuildType: params.BuildType,
		Status:    logic.StatusStop,
		Message:   "",
	}
	imageRow, _ := dao.Image.Where(dao.Image.ID.Eq(params.Id)).First()
	if imageRow == nil {
		imageRow = imageNew

	} else {
		// 如果已经构建过，先查找一下旧镜像，新加一个标签，避免变成 none 标签
		_, _, err := docker.Sdk.Client.ImageInspectWithRaw(docker.Sdk.Ctx, imageName)
		if err == nil {
			_ = docker.Sdk.Client.ImageTag(docker.Sdk.Ctx, imageName, imageName+"-deprecated-"+function.GetRandomString(6))
		}
		dao.Image.Select(
			dao.Image.Status,
			dao.Image.Message,
			dao.Image.Tag,
			dao.Image.Setting,
		).Where(dao.Image.ID.Eq(imageRow.ID)).Updates(imageNew)
	}
	buildImageTask.ImageId = imageRow.ID

	buildImageTask.Platform = &logic.Platform{
		Type: params.Platform,
		Arch: params.PlatformArch,
	}

	err := logic.DockerTask{}.ImageBuild(buildImageTask)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	dao.Image.Create(imageRow)

	self.JsonResponseWithoutError(http, gin.H{
		"imageId": imageRow.ID,
	})
	return
}

func (self Image) GetList(http *gin.Context) {
	type ParamsValidate struct {
		Tag   string `form:"tag" binding:"omitempty"`
		Title string `json:"title"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	var filterTagList []string
	if params.Title != "" {
		_ = dao.Image.Where(dao.Image.Title.Like("%"+params.Title+"%")).Pluck(dao.Image.Tag, &filterTagList)
	}
	if params.Tag != "" {
		filterTagList = append(filterTagList, params.Tag)
	}

	var result []image.Summary
	imageList, err := docker.Sdk.Client.ImageList(docker.Sdk.Ctx, image.ListOptions{
		All:            false,
		ContainerCount: true,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	containerList, _ := docker.Sdk.Client.ContainerList(docker.Sdk.Ctx, container.ListOptions{
		All: true,
	})

	for key, summary := range imageList {
		if imageList[key].Containers == -1 {
			imageList[key].Containers = 0
			for _, cItem := range containerList {
				if cItem.ImageID == summary.ID {
					imageList[key].Containers += 1
				}
			}
		}

		if len(summary.RepoTags) == 0 {
			if len(summary.RepoDigests) != 0 {
				noneTag := strings.Split(summary.RepoDigests[0], "@")
				imageList[key].RepoTags = append(summary.RepoTags, noneTag[0]+":none")
			} else {
				imageList[key].RepoTags = append(summary.RepoTags, "none")
			}
		}
	}

	if !function.IsEmptyArray(filterTagList) {
		for _, summary := range imageList {
			for _, tag := range summary.RepoTags {
				has := false
				for _, s := range filterTagList {
					if strings.Contains(tag, s) {
						has = true
						result = append(result, summary)
						break
					}
				}
				if has {
					break
				}
			}
		}
	} else {
		result = imageList
	}

	self.JsonResponseWithoutError(http, gin.H{
		"list": result,
	})
	return
}

func (self Image) GetDetail(http *gin.Context) {
	type ParamsValidate struct {
		Md5 string `form:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	layer, err := docker.Sdk.Client.ImageHistory(docker.Sdk.Ctx, params.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	imageDetail, _, err := docker.Sdk.Client.ImageInspectWithRaw(docker.Sdk.Ctx, params.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"layer": layer,
		"info":  imageDetail,
	})
	return
}

func (self Image) ImageDelete(http *gin.Context) {
	type ParamsValidate struct {
		Md5   []string `json:"md5" binding:"required"`
		Force bool     `json:"force"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	if !function.IsEmptyArray(params.Md5) {
		for _, sha := range params.Md5 {
			_, err := docker.Sdk.Client.ImageRemove(docker.Sdk.Ctx, sha, image.RemoveOptions{
				PruneChildren: true,
				Force:         params.Force,
			})
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
		}
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Image) ImagePrune(http *gin.Context) {
	filter := filters.NewArgs()
	filter.Add("dangling", "0")
	docker.Sdk.Client.ImagesPrune(docker.Sdk.Ctx, filter)
	self.JsonSuccessResponse(http)
	return
}

func (self Image) BuildPrune(http *gin.Context) {
	_, err := docker.Sdk.Client.BuildCachePrune(docker.Sdk.Ctx, types.BuildCachePruneOptions{
		All: true,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Image) Export(http *gin.Context) {
	type ParamsValidate struct {
		Md5 string `json:"md5" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	imageInfo, _, err := docker.Sdk.Client.ImageInspectWithRaw(docker.Sdk.Ctx, params.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	out, err := docker.Sdk.Client.ImageSave(docker.Sdk.Ctx, imageInfo.RepoTags)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer out.Close()
	tempFile, _ := os.CreateTemp("", "dpanel")
	defer tempFile.Close()
	defer os.Remove(tempFile.Name())
	_, err = io.Copy(tempFile, out)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	http.File(tempFile.Name())
	return
}
