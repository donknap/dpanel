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
		Platform string `json:"platform"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	var authString string
	tagDetail := logic.Image{}.GetImageTagDetail(params.Tag)

	proxyList := make([]string, 0)
	// 从官方仓库拉取镜像不用权限
	registry, _ := dao.Registry.Where(dao.Registry.ServerAddress.Eq(tagDetail.Registry)).Find()
	for _, registryRow := range registry {
		// 类似于腾讯云，同一个仓库地址，但是可能根据不同的命令空间指定权限
		if registryRow.Setting.Username == tagDetail.Namespace {
			authString = logic.Image{}.GetRegistryAuthString(registryRow.ServerAddress, registryRow.Setting.Username, registryRow.Setting.Password)
		}
		proxyList = append(proxyList, registryRow.Setting.Proxy...)
	}

	if authString == "" && registry != nil && len(registry) > 0 {
		authString = logic.Image{}.GetRegistryAuthString(registry[0].ServerAddress, registry[0].Setting.Username, registry[0].Setting.Password)
	}

	var proxyUrl string

	if params.AsLatest {
		tag := strings.Split(params.Tag, ":")
		latestTag := tag[0] + ":latest"
		err := docker.Sdk.Client.ImageTag(docker.Sdk.Ctx, params.Tag, latestTag)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		err = logic.DockerTask{}.ImageRemote(&logic.ImageRemoteOption{
			Auth: authString,
			Type: params.Type,
			Tag:  latestTag,
		})
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	} else {
		// 如果仓库采用了代理地址，则优先使用，代理地址无法拉取后，尝试用原始的地址拉取
		if !function.IsEmptyArray(proxyList) {
			proxyList = append(proxyList, tagDetail.Registry)
			var err error
			for _, value := range proxyList {
				proxy := strings.Trim(strings.TrimPrefix(value, "https://"), "/")
				if tagDetail.Registry == "docker.io" && strings.Count(tagDetail.ImageName, "/") == 0 {
					proxy += "/library"
				}
				err = logic.DockerTask{}.ImageRemote(&logic.ImageRemoteOption{
					Auth:     authString,
					Type:     params.Type,
					Tag:      params.Tag,
					Proxy:    proxy,
					Platform: params.Platform,
				})
				if err == nil {
					proxyUrl = value
					// 如果使用了加速，需要给镜像 tag 一个原来的名称
					_ = docker.Sdk.Client.ImageTag(docker.Sdk.Ctx, proxy+"/"+tagDetail.Tag, params.Tag)
					break
				} else {
					if strings.Contains(err.Error(), "not found:") {
						self.JsonResponseWithError(http, err, 500)
						return
					}
				}
			}
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
		} else {
			err := logic.DockerTask{}.ImageRemote(&logic.ImageRemoteOption{
				Auth:     authString,
				Type:     params.Type,
				Tag:      params.Tag,
				Platform: params.Platform,
			})
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
		}
	}
	self.JsonResponseWithoutError(http, gin.H{
		"proxyUrl": proxyUrl,
		"tag":      tagDetail.Tag,
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
		authString := logic.Image{}.GetRegistryAuthString(registry.ServerAddress, registry.Setting.Username, registry.Setting.Password)
		for _, md5 := range params.Md5 {
			imageDetail, _, err := docker.Sdk.Client.ImageInspectWithRaw(docker.Sdk.Ctx, md5)
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
			for _, tag := range imageDetail.RepoTags {
				imageName := logic.Image{}.GetImageTagDetail(tag)
				newImageName := logic.Image{}.GetImageName(&logic.ImageNameOption{
					Registry:  registry.ServerAddress,
					Name:      imageName.ImageName,
					Namespace: params.NewNamespace,
				})
				if !function.InArray(imageDetail.RepoTags, newImageName) {
					err = docker.Sdk.Client.ImageTag(docker.Sdk.Ctx, imageName.ImageName, newImageName)
					if err != nil {
						self.JsonResponseWithError(http, err, 500)
						return
					}
				}
				err = logic.DockerTask{}.ImageRemote(&logic.ImageRemoteOption{
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
	}

	self.JsonSuccessResponse(http)
	return
}
