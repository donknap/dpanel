package controller

import (
	"errors"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go-support/src/facade"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
	"io"
	"os"
	"strings"
)

type Image struct {
	controller.Abstract
}

func (self Image) CreateByDockerfile(http *gin.Context) {
	type ParamsValidate struct {
		Id              int32  `form:"id"`
		Registry        string `form:"registry"`
		Tag             string `form:"tag" binding:"required"`
		BuildType       string `form:"buildType" binding:"required"`
		BuildDockerfile string `form:"buildDockerfile" binding:"omitempty"`
		BuildGit        string `form:"buildGit" binding:"omitempty"`
		BuildZip        string `form:"buildZip" binding:"omitempty"`
		BuildRoot       string `form:"buildRoot" binding:"omitempty"`
		BuildTemplate   string `form:"buildTemplate" binding:"omitempty"`
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
		self.JsonResponseWithError(http, errors.New("Zip 包和 Git 地址只需要只定一项"), 500)
		return
	}
	mustHasZipFile := false
	buildImageTask := &logic.BuildImageMessage{
		Tag: params.Tag,
	}
	if params.Registry != "" {
		buildImageTask.Tag = params.Registry + "/" + params.Tag
	}
	if params.BuildRoot != "" {
		buildImageTask.Context = "./" + strings.Trim(strings.Trim(strings.Trim(params.BuildRoot, ""), "./"), "/") + "/Dockerfile"
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
		Registry:        params.Registry,
		Tag:             params.Tag,
		BuildGit:        params.BuildGit,
		BuildDockerfile: params.BuildDockerfile,
		BuildZip:        params.BuildZip,
		BuildRoot:       params.BuildRoot,
		BuildType:       params.BuildType,
		BuildTemplate:   params.BuildTemplate,
		Status:          logic.STATUS_STOP,
		Message:         "",
	}
	imageRow, _ := dao.Image.Where(dao.Image.ID.Eq(params.Id)).First()
	if imageRow == nil {
		imageRow = imageNew
		dao.Image.Create(imageRow)
	} else {
		if params.BuildZip != imageRow.BuildZip {
			storage.Local{}.Delete(imageRow.BuildZip)
		}
		dao.Image.Select(
			dao.Image.BuildDockerfile,
			dao.Image.BuildRoot,
			dao.Image.BuildZip,
			dao.Image.BuildGit,
			dao.Image.Status,
			dao.Image.Message,
			dao.Image.Registry,
			dao.Image.Tag,
		).Where(dao.Image.ID.Eq(imageRow.ID)).Updates(imageNew)
	}
	buildImageTask.ImageId = imageRow.ID

	err := logic.DockerTask{}.ImageBuild(buildImageTask)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"imageId": imageRow.ID,
	})
	return
}

func (self Image) GetList(http *gin.Context) {
	type ParamsValidate struct {
		Tag string `form:"tag" binding:"omitempty"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	var result []types.ImageSummary

	imageList, err := docker.Sdk.Client.ImageList(docker.Sdk.Ctx, types.ImageListOptions{
		All:            false,
		ContainerCount: true,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	for key, summary := range imageList {
		if len(summary.RepoTags) == 0 {
			if len(summary.RepoDigests) != 0 {
				noneTag := strings.Split(summary.RepoDigests[0], "@")
				imageList[key].RepoTags = append(summary.RepoTags, noneTag[0]+":none")
			} else {
				imageList[key].RepoTags = append(summary.RepoTags, "none")
			}
		}
	}

	if params.Tag != "" {
		for _, summary := range imageList {
			if !function.IsEmptyArray(summary.RepoTags) {
				for _, tag := range summary.RepoTags {
					if strings.Contains(tag, params.Tag) {
						result = append(result, summary)
						break
					}
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

func (self Image) GetListBuild(http *gin.Context) {
	type ParamsValidate struct {
		Page     int `form:"page,default=1" binding:"omitempty,gt=0"`
		PageSize int `form:"pageSize" binding:"omitempty"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 10
	}

	query := dao.Image.Order(dao.Image.ID.Desc())
	list, total, _ := query.FindByPage((params.Page-1)*params.PageSize, params.PageSize)
	self.JsonResponseWithoutError(http, gin.H{
		"total": total,
		"page":  params.Page,
		"list":  list,
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

func (self Image) Remote(http *gin.Context) {
	type ParamsValidate struct {
		Tag      string `json:"tag" binding:"required"`
		Type     string `json:"type" binding:"required,oneof=pull push"`
		AsLatest bool   `json:"asLatest"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	var authString string
	tagArr := strings.Split(params.Tag, "/")
	registry, _ := dao.Registry.Where(dao.Registry.ServerAddress.Eq(tagArr[0])).First()
	if registry != nil {
		password, _ := function.AseDecode(facade.GetConfig().GetString("app.name"), registry.Password)
		authString = function.Base64Encode(struct {
			Username      string `json:"username"`
			Password      string `json:"password"`
			ServerAddress string `json:"serveraddress"`
		}{
			Username:      registry.Username,
			Password:      password,
			ServerAddress: registry.ServerAddress,
		})
	}
	if params.AsLatest {
		tag := strings.Split(params.Tag, ":")
		latestTag := tag[0] + ":latest"
		err := docker.Sdk.Client.ImageTag(docker.Sdk.Ctx, params.Tag, latestTag)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		err = logic.DockerTask{}.ImageRemote(&logic.ImageRemoteMessage{
			Auth: authString,
			Type: params.Type,
			Tag:  latestTag,
		})
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	} else {
		err := logic.DockerTask{}.ImageRemote(&logic.ImageRemoteMessage{
			Auth: authString,
			Type: params.Type,
			Tag:  params.Tag,
		})
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	self.JsonResponseWithoutError(http, gin.H{
		"tag": params.Tag,
	})
	return
}

func (self Image) TagDelete(http *gin.Context) {
	type ParamsValidate struct {
		Tag   string `form:"tag" binding:"required"`
		Force bool   `form:"force" binding:"omitempty"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	_, err := docker.Sdk.Client.ImageRemove(docker.Sdk.Ctx, params.Tag, types.ImageRemoveOptions{
		Force: params.Force,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"tag": params.Tag,
	})
	return
}

func (self Image) TagAdd(http *gin.Context) {
	type ParamsValidate struct {
		Md5 string `form:"md5" binding:"required"`
		Tag string `form:"tag" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	imageDetail, _, err := docker.Sdk.Client.ImageInspectWithRaw(docker.Sdk.Ctx, params.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if function.InArray[string](imageDetail.RepoTags, params.Tag) {
		self.JsonResponseWithError(http, errors.New("该标签已经存在"), 500)
		return
	}
	err = docker.Sdk.Client.ImageTag(docker.Sdk.Ctx, imageDetail.RepoTags[0], params.Tag)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"tag": params.Tag,
	})
	return
}

func (self Image) GetImageTask(http *gin.Context) {
	type ParamsValidate struct {
		Id int32 `form:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	imageRow, _ := dao.Image.Where(dao.Image.ID.Eq(params.Id)).First()
	if imageRow == nil {
		self.JsonResponseWithError(http, errors.New("构建任务不存在"), 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"task": imageRow,
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
			_, err := docker.Sdk.Client.ImageRemove(docker.Sdk.Ctx, sha, types.ImageRemoveOptions{
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
