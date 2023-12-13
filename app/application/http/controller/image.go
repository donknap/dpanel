package controller

import (
	"errors"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go-support/src/facade"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
	"io"
	"net/url"
	"os"
	"strings"
)

type Image struct {
	controller.Abstract
}

func (self Image) CreateByDockerfile(http *gin.Context) {
	type ParamsValidate struct {
		Tag        string `form:"tag" binding:"required"`
		DockerFile string `form:"dockerFile" binding:"omitempty"`
		ZipFile    string `form:"zipFile" binding:"omitempty,required_without=DockerFile"`
		Git        string `form:"git" binding:"omitempty"`
		Context    string `form:"context" binding:"omitempty"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	if params.DockerFile == "" && params.ZipFile == "" && params.Git == "" {
		self.JsonResponseWithError(http, errors.New("至少需要指定 Dockerfile、Zip 包或是 Git 地址"), 500)
		return
	}
	if params.ZipFile != "" && params.Git != "" {
		self.JsonResponseWithError(http, errors.New("Zip 包和 Git 地址只需要只定一项"), 500)
		return
	}
	mustHasZipFile := false
	buildImageTask := &logic.BuildImageMessage{
		Tag:     params.Tag,
		Context: "./" + strings.Trim(strings.Trim(params.Context, "./"), "/") + "/Dockerfile",
	}
	addStr := []string{
		"ADD",
		"COPY",
	}
	for _, str := range addStr {
		if strings.Contains(strings.ToUpper(params.DockerFile), str) {
			mustHasZipFile = true
		}
	}

	if mustHasZipFile {
		if params.ZipFile == "" && params.Git == "" {
			self.JsonResponseWithError(http, errors.New("Dockerfile中包含添加文件操作，请上传对应的Zip包或是指定Git仓库"), 500)
			return
		}
	}
	if params.ZipFile != "" {
		path := os.TempDir() + "/" + params.ZipFile
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			self.JsonResponseWithError(http, errors.New("请先上传压缩包"), 500)
			return
		}
		buildImageTask.ZipPath = path
	}
	if params.DockerFile != "" {
		buildImageTask.DockerFileContent = []byte(params.DockerFile)
	}
	if params.Git != "" {
		buildImageTask.GitUrl = params.Git
	}

	imageRow := &entity.Image{
		Tag:             params.Tag,
		BuildGit:        params.Git,
		BuildDockerfile: params.DockerFile,
		BuildZip:        params.ZipFile,
		Status:          logic.STATUS_STOP,
		StatusStep:      "",
		Message:         "",
	}
	dao.Image.Create(imageRow)
	buildImageTask.ImageId = imageRow.ID

	task := logic.NewDockerTask()
	task.QueueBuildImage <- buildImageTask

	self.JsonResponseWithoutError(http, gin.H{
		"imageId": imageRow.ID,
	})
	return
}

func (self Image) GetList(http *gin.Context) {
	type ParamsValidate struct {
		Type string `form:"type" binding:"required,oneof=all self"`
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

	if params.Type == "self" {
		query := dao.Image.Order(dao.Image.ID.Desc())
		switch params.Type {
		case "build":
			query = query.Where(dao.Image.Status.In(logic.STATUS_STOP, logic.STATUS_PROCESSING, logic.STATUS_ERROR))
			break
		case "self":
			query = query.Where(dao.Image.Status.In(logic.STATUS_SUCCESS))
			break
		}
		list, _ := query.Find()
		if list == nil {
			self.JsonResponseWithoutError(http, gin.H{
				"list": list,
			})
			return
		}
		for _, image := range list {
			for _, summary := range imageList {
				if image.Md5 == summary.ID {
					result = append(result, summary)
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
		Id string `form:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	layer, err := docker.Sdk.Client.ImageHistory(docker.Sdk.Ctx, params.Id)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	imageDetail, _, err := docker.Sdk.Client.ImageInspectWithRaw(docker.Sdk.Ctx, params.Id)
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
		Tag  string `form:"tag" binding:"required"`
		Type string `form:"type" binding:"required,oneof=pull push"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	urls, err := url.Parse("https://" + params.Tag)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	registry, _ := dao.Registry.Where(dao.Registry.ServerAddress.Eq(urls.Host)).First()
	if registry == nil {
		self.JsonResponseWithError(http, errors.New(fmt.Sprintf("推送前请先添加 %s 仓库的权限", urls.Host)), 500)
		return
	}
	password, _ := function.AseDecode(facade.GetConfig().GetString("app.name"), registry.Password)
	authString := function.Base64Encode(struct {
		Username      string `json:"username"`
		Password      string `json:"password"`
		ServerAddress string `json:"serveraddress"`
	}{
		Username:      registry.Username,
		Password:      password,
		ServerAddress: registry.ServerAddress,
	})

	go func() {
		var out io.ReadCloser
		if params.Type == "pull" {
			out, err = docker.Sdk.Client.ImagePull(docker.Sdk.Ctx, params.Tag, types.ImagePullOptions{
				RegistryAuth: authString,
			})
		} else {
			out, err = docker.Sdk.Client.ImagePush(docker.Sdk.Ctx, params.Tag, types.ImagePushOptions{
				RegistryAuth: authString,
			})
		}
		if err != nil {
			notice.Message{}.Error(params.Type, err.Error())
			return
		}
		progressChan := docker.Sdk.Progress(out)
		for {
			select {
			case message, ok := <-progressChan:
				if !ok {
					notice.Message{}.Success(params.Type, params.Tag)
					return
				}
				if message.Err != nil {
					notice.Message{}.Error(params.Type, message.Err.Error())
					return
				}
			}
		}
	}()
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
