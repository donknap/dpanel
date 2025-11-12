package controller

import (
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	dockerRegistry "github.com/docker/docker/api/types/registry"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/registry"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/gin-gonic/gin"
)

func (self Image) TagSync(http *gin.Context) {
	type ParamsValidate struct {
		Tag      string `json:"tag" binding:"required"`
		Type     string `json:"type" binding:"required,oneof=pull push"`
		Platform string `json:"platform"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	imageNameDetail := function.ImageTag(params.Tag)
	registryConfig := logic.Image{}.GetRegistryConfig(imageNameDetail.Registry)

	var out io.ReadCloser
	var err error

	slog.Debug("image remote", "type", params.Type, "tag", imageNameDetail.Uri())

	wsBuffer := ws.NewProgressPip(fmt.Sprintf(ws.MessageTypeImagePull, params.Tag))
	defer wsBuffer.Close()

	for _, s := range registryConfig.Address {
		if params.Type == "pull" {
			imageNameDetail.Registry = s
			// 使用代理地址时先尝试发起 http 请求是否可以访问。以便于可以减少直接 pull 的等待时间
			reg := registry.New(
				registry.WithAddress(s),
				registry.WithCredentialsWithBasic(registryConfig.Config.Username, registryConfig.Config.Password),
			)
			if err := reg.Client().Ping(); err != nil {
				slog.Debug("image remote select registry url", "error", err)
				continue
			}
			pullOption := image.PullOptions{
				RegistryAuth: registryConfig.GetAuthString(),
			}
			if params.Platform != "" {
				pullOption.Platform = params.Platform
			}
			out, err = docker.Sdk.Client.ImagePull(wsBuffer.Context(), imageNameDetail.Uri(), pullOption)
			if err != nil {
				slog.Debug("image remote pull", "error", err)
			}
		} else {
			// 推荐送镜像时保持原样
			// 自建仓库不需要添加 library
			// 即使推送 hub 镜像，library 命名空间属于官方空间，也不应该添加
			out, err = docker.Sdk.Client.ImagePush(docker.Sdk.Ctx, params.Tag, image.PushOptions{
				RegistryAuth: registryConfig.GetAuthString(),
			})
		}
		if err == nil {
			break
		}
	}

	// 可能最后循环后还包含错误
	if err != nil {
		if function.ErrorHasKeyword(err, "not found:", "repository does not exist") {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageImagePullTagNotFound, "tag", params.Tag), 500)
			return
		}
		if function.ErrorHasKeyword(err, "server gave HTTP response to HTTPS client") {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageImagePullServerHttp), 500)
			return
		}
		self.JsonResponseWithError(http, err, 500)
		return
	}

	if params.Type == "pull" {
		_ = notice.Message{}.Info(".imagePull", "name", imageNameDetail.Uri())
	}

	err = logic.DockerTask{}.ImageRemote(wsBuffer, out)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	if params.Type == "pull" {
		// 如果使用了加速，需要给镜像 tag 一个原来的名称
		// 当 tag 中包含 @ digest 值时，不能直接 tag 成新名称，需要获取到其实中的版本号
		if tag, _, ok := strings.Cut(params.Tag, "@"); ok {
			params.Tag = tag
		}
		err = docker.Sdk.Client.ImageTag(docker.Sdk.Ctx, imageNameDetail.Uri(), params.Tag)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		// 不能取消掉原有的镜像文件会导致 digest 丢失
		//oldImageNameDetail := registry2.GetImageTagDetail(params.Tag)
		//
		//if oldImageNameDetail.Registry != imageNameDetail.Registry {
		//	_, err = docker.Sdk.Client.ImageRemove(docker.Sdk.Ctx, imageNameDetail.Uri(), image.RemoveOptions{})
		//	if err != nil {
		//		slog.Debug("image remote tag", "error", err)
		//	}
		//}
	}

	self.JsonResponseWithoutError(http, gin.H{
		"proxyUrl": imageNameDetail.Registry,
		"tag":      params.Tag,
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
	self.JsonSuccessResponse(http)
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

	imageDetail, err := docker.Sdk.Client.ImageInspect(docker.Sdk.Ctx, params.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if function.InArray[string](imageDetail.RepoTags, params.Tag) {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonIdAlreadyExists, "name", params.Tag), 500)
		return
	}

	err = docker.Sdk.Client.ImageTag(docker.Sdk.Ctx, imageDetail.ID, params.Tag)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Image) TagPushBatch(http *gin.Context) {
	type ParamsValidate struct {
		Md5                   []string `json:"md5" binding:"required"`
		RegistryServerAddress []string `json:"registryServerAddress" binding:"required"`
		NewNamespace          string   `json:"newNamespace"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	for _, address := range params.RegistryServerAddress {
		registryConfig := logic.Image{}.GetRegistryConfig(address)
		imagePushOption := image.PushOptions{
			RegistryAuth: registryConfig.GetAuthString(),
		}
		for _, md5 := range params.Md5 {
			imageDetail, err := docker.Sdk.Client.ImageInspect(docker.Sdk.Ctx, md5)
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
			for _, tag := range imageDetail.RepoTags {
				newImageName := function.ImageTag(tag)
				newImageName.Registry = address
				newImageName.Namespace = params.NewNamespace
				if !function.InArray(imageDetail.RepoTags, newImageName.Uri()) {
					err = docker.Sdk.Client.ImageTag(docker.Sdk.Ctx, tag, newImageName.Uri())
					if err != nil {
						self.JsonResponseWithError(http, err, 500)
						return
					}
				}
				out, err := docker.Sdk.Client.ImagePush(docker.Sdk.Ctx, newImageName.Uri(), imagePushOption)
				if err != nil {
					self.JsonResponseWithError(http, err, 500)
					return
				}
				_, err = io.Copy(io.Discard, out)
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

func (self Image) TagSearch(http *gin.Context) {
	type ParamsValidate struct {
		Keyword string `json:"keyword" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	list, err := docker.Sdk.Client.ImageSearch(docker.Sdk.Ctx, params.Keyword, dockerRegistry.SearchOptions{
		RegistryAuth:  "",
		PrivilegeFunc: nil,
		Filters:       filters.Args{},
		Limit:         0,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"list": list,
	})
	return
}
