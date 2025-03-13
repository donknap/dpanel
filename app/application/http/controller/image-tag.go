package controller

import (
	"errors"
	"fmt"
	"github.com/docker/docker/api/types/image"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/notice"
	registry2 "github.com/donknap/dpanel/common/service/registry"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"io"
	"log/slog"
	http2 "net/http"
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
	var response *http2.Response

	slog.Debug("image remote", "type", params.Type, "tag", imageNameDetail.Uri())

	if params.Type == "pull" {
		_ = notice.Message{}.Info(".imagePull", "name", params.Tag)
	}

	wsBuffer := ws.NewProgressPip(fmt.Sprintf(ws.MessageTypeImagePull, params.Tag))
	defer wsBuffer.Close()

	for i, s := range registryConfig.Proxy {
		imageNameDetail.Registry = s
		if params.Type == "pull" {
			// 仓库地址如果大于1，则表示有代理地址，最后一个为本地址
			// 仅当是代理地址的时候才检测是否可以访问
			// 拉取时先检测一下当前仓库地址是否可以访问，并且判断一下是否是 docker hub 的代理地址
			if i == len(registryConfig.Proxy)-1 {
				pullOption := image.PullOptions{
					RegistryAuth: registryConfig.GetAuthString(),
				}
				if params.Platform != "" {
					pullOption.Platform = params.Platform
				}
				out, err = docker.Sdk.Client.ImagePull(wsBuffer.Context(), imageNameDetail.Uri(), pullOption)
			} else {
				url := registry2.GetRegistryUrl(s)
				if response, err = http2.Get(strings.Replace(url.String(), "https://", "http://", 1)); err != nil {
					if response != nil {
						slog.Debug("image remote select registry url", "header", response.Header.Get(registry2.ChallengeHeader))
					}
					slog.Debug("image remote select registry url", "error", err)
					continue
				}
				pullOption := image.PullOptions{
					RegistryAuth: registryConfig.GetAuthString(),
				}
				if params.Platform != "" {
					pullOption.Platform = params.Platform
				}
				slog.Debug("image remote proxy", "tag", imageNameDetail.Uri())

				out, err = docker.Sdk.Client.ImagePull(wsBuffer.Context(), imageNameDetail.Uri(), pullOption)
				if err != nil {
					slog.Debug("image remote pull", "error", err)
				}
			}
		} else {
			// 推荐送镜像时保持原样
			// 自建仓库不需要添加 library
			// 即使推送 hub 镜像，library 命名空间属于官司方空间，也不应该添加
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
			self.JsonResponseWithError(http, notice.Message{}.New(".imagePullTagNotFound", "tag", params.Tag), 500)
			return
		}
		if function.ErrorHasKeyword(err, "server gave HTTP response to HTTPS client") {
			self.JsonResponseWithError(http, notice.Message{}.New(".imagePullServerHttp"), 500)
			return
		}
		self.JsonResponseWithError(http, err, 500)
		return
	}

	err = logic.DockerTask{}.ImageRemote(wsBuffer, out)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	if params.Type == "pull" {
		// 如果使用了加速，需要给镜像 tag 一个原来的名称
		_ = notice.Message{}.Info(".imagePullUseProxy", "name", imageNameDetail.Registry)

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

	imageDetail, err := docker.Sdk.Client.ImageInspect(docker.Sdk.Ctx, params.Md5)
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
		password := ""
		if registry.Setting != nil && registry.Setting.Username != "" && registry.Setting.Password != "" {
			password, _ = function.AseDecode(facade.GetConfig().GetString("app.name"), registry.Setting.Password)
		}

		registryConfig := registry2.Config{
			Username: registry.Setting.Username,
			Password: password,
			Host:     registry.ServerAddress,
		}

		for _, md5 := range params.Md5 {
			imageDetail, err := docker.Sdk.Client.ImageInspect(docker.Sdk.Ctx, md5)
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
			for _, tag := range imageDetail.RepoTags {
				newImageName := registry2.GetImageTagDetail(tag)
				newImageName.Registry = registry.ServerAddress
				newImageName.Namespace = params.NewNamespace

				if !function.InArray(imageDetail.RepoTags, newImageName.Uri()) {
					fmt.Printf("%v \n", newImageName.Uri())
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
