package controller

import (
	"errors"
	"github.com/docker/docker/api/types/image"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/notice"
	registry2 "github.com/donknap/dpanel/common/service/registry"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"io"
	"strings"
)

func (self Image) TagRemote(http *gin.Context) {
	type ParamsValidate struct {
		Tag      string `json:"tag" binding:"required"`
		Type     string `json:"type" binding:"required,oneof=pull push"`
		Platform string `json:"platform"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	imageNameDetail := registry2.GetImageTagDetail(params.Tag)
	registryConfig := logic.Image{}.GetRegistryConfig(imageNameDetail.Uri())

	var out io.ReadCloser
	var err error

	for _, s := range registryConfig.Proxy {
		imageNameDetail.Registry = s

		if params.Type == "pull" {
			pullOption := image.PullOptions{
				RegistryAuth: registryConfig.GetAuthString(),
			}
			if params.Platform != "" {
				pullOption.Platform = params.Platform
			}
			out, err = docker.Sdk.Client.ImagePull(docker.Sdk.Ctx, imageNameDetail.Uri(), pullOption)
		} else {
			out, err = docker.Sdk.Client.ImagePush(docker.Sdk.Ctx, imageNameDetail.Uri(), image.PushOptions{
				RegistryAuth: registryConfig.GetAuthString(),
			})
		}

		if err == nil {
			break
		}
	}

	// 可能最后循环后还包含错误
	if err != nil {
		if strings.Contains(err.Error(), "not found:") {
			self.JsonResponseWithError(http, errors.New(".imagePullNotFound"), 500)
			return
		}
		self.JsonResponseWithError(http, err, 500)
		return
	}

	err = logic.DockerTask{}.ImageRemote(params.Tag, out)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	if params.Type == "pull" {
		// 如果使用了加速，需要给镜像 tag 一个原来的名称
		_ = notice.Message{}.Info("imagePullUseProxy", "name", imageNameDetail.Registry)
		_ = docker.Sdk.Client.ImageTag(docker.Sdk.Ctx, imageNameDetail.Uri(), params.Tag)
	}

	self.JsonResponseWithoutError(http, gin.H{
		"proxyUrl": imageNameDetail.Registry,
		"tag":      imageNameDetail.ImageName,
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
	_ = notice.Message{}.Info("imageTagCreate", params.Tag)
	self.JsonResponseWithoutError(http, gin.H{
		"tag": params.Tag,
	})
	return
}

func (self Image) TagSync(http *gin.Context) {
	type ParamsValidate struct {
		Md5          []string `json:"md5" binding:"required"`
		RegistryId   []int32  `json:"registryId" binding:"required"`
		NewNamespace string   `json:"newNamespace"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if function.IsEmptyArray(params.Md5) {
		self.JsonResponseWithError(http, errors.New("请选择要推送的镜像"), 500)
		return
	}

	for _, id := range params.RegistryId {
		registry, _ := dao.Registry.Where(dao.Registry.ID.Eq(id)).First()
		if registry == nil {
			self.JsonResponseWithError(http, errors.New("仓库不存在"), 500)
			return
		}

		password, _ := function.AseDecode(facade.GetConfig().GetString("app.name"), registry.Setting.Password)
		registryConfig := registry2.Config{
			Username: registry.Setting.Username,
			Password: password,
			Host:     registry.ServerAddress,
		}

		for _, md5 := range params.Md5 {
			imageDetail, _, err := docker.Sdk.Client.ImageInspectWithRaw(docker.Sdk.Ctx, md5)
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
			for _, tag := range imageDetail.RepoTags {
				newImageName := registry2.GetImageTagDetail(tag)
				newImageName.Registry = registry.ServerAddress
				newImageName.Namespace = params.NewNamespace

				if !function.InArray(imageDetail.RepoTags, newImageName.Uri()) {
					err = docker.Sdk.Client.ImageTag(docker.Sdk.Ctx, tag, newImageName.Uri())
					if err != nil {
						self.JsonResponseWithError(http, err, 500)
						return
					}
				}
				out, err := docker.Sdk.Client.ImagePush(docker.Sdk.Ctx, newImageName.Uri(), image.PushOptions{
					RegistryAuth: registryConfig.GetAuthString(),
				})
				if err != nil {
					self.JsonResponseWithError(http, err, 500)
					return
				}
				err = logic.DockerTask{}.ImageRemote(tag, out)
				if err != nil {
					self.JsonResponseWithError(http, err, 500)
					return
				}
			}
		}
	}

	self.JsonSuccessResponse(http)
	return
}
