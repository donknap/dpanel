package controller

import (
	"errors"
	"github.com/docker/docker/api/types/image"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
	"strings"
)

func (self Image) TagRemote(http *gin.Context) {
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
	tagDetail := logic.Image{}.GetImageTagDetail(params.Tag)
	registry, _ := dao.Registry.Where(dao.Registry.ServerAddress.Eq(tagDetail.Registry)).Find()
	for _, registryRow := range registry {
		if registryRow.Username == tagDetail.Namespace {
			authString = logic.Image{}.GetRegistryAuthString(registryRow.ServerAddress, registryRow.Username, registryRow.Password)
		}
	}

	if authString == "" && registry != nil && len(registry) > 0 {
		authString = logic.Image{}.GetRegistryAuthString(registry[0].ServerAddress, registry[0].Username, registry[0].Password)
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
		// 从官方仓库拉取镜像不用权限
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
	_, err := docker.Sdk.Client.ImageRemove(docker.Sdk.Ctx, params.Tag, image.RemoveOptions{
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
	err = docker.Sdk.Client.ImageTag(docker.Sdk.Ctx, imageDetail.ID, params.Tag)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"tag": params.Tag,
	})
	return
}

func (self Image) TagSync(http *gin.Context) {
	type ParamsValidate struct {
		Tag        []string `json:"tag" binding:"required"`
		RegistryId []int32  `json:"registryId" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if function.IsEmptyArray(params.Tag) {
		self.JsonResponseWithError(http, errors.New("请选择要推送的镜像"), 500)
		return
	}
	for _, id := range params.RegistryId {
		registry, _ := dao.Registry.Where(dao.Registry.ID.Eq(id)).First()
		if registry == nil {
			self.JsonResponseWithError(http, errors.New("仓库不存在"), 500)
			return
		}
		authString := logic.Image{}.GetRegistryAuthString(registry.ServerAddress, registry.Username, registry.Password)
		for _, tag := range params.Tag {
			imageDetail, _, err := docker.Sdk.Client.ImageInspectWithRaw(docker.Sdk.Ctx, tag)
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
			imageName := logic.Image{}.GetImageTagDetail(imageDetail.RepoTags[0])
			newImageName := logic.Image{}.GetImageName(&logic.ImageNameOption{
				Registry: registry.ServerAddress,
				Name:     imageName.ImageName,
			})
			err = docker.Sdk.Client.ImageTag(docker.Sdk.Ctx, imageName.ImageName, newImageName)
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
			err = logic.DockerTask{}.ImageRemote(&logic.ImageRemoteMessage{
				Auth: authString,
				Type: "push",
				Tag:  newImageName,
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
