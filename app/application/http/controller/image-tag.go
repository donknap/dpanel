package controller

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	dockerRegistry "github.com/docker/docker/api/types/registry"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	dockerTypes "github.com/donknap/dpanel/common/service/docker/types"
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

	var err error

	dockerClient, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	var wsBuffer *ws.ProgressPip
	owner := true
	if params.Type == "pull" {
		wsBuffer, owner, err = ws.NewFdProgressPip(http, dockerClient.Name, fmt.Sprintf(ws.MessageTypeImagePull, params.Tag))
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		if !owner {
			if err = wsBuffer.Wait(http.Request.Context()); err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
			self.JsonResponseWithoutError(http, gin.H{
				"proxyUrl": imageNameDetail.Registry,
				"tag":      params.Tag,
			})
			return
		}
	} else {
		wsBuffer = ws.NewProgressPip(fmt.Sprintf(ws.MessageTypeImagePull, params.Tag))
	}

	defer wsBuffer.Close()
	operationCtx, cancelOperation := context.WithCancel(dockerClient.Ctx)
	defer cancelOperation()
	stopWatchProgress := context.AfterFunc(wsBuffer.Context(), cancelOperation)
	defer stopWatchProgress()

	oldImage, err := dockerClient.Client.ImageInspect(operationCtx, params.Tag)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if params.Type == "pull" {
		_, err = dockerClient.ImagePull(operationCtx, params.Tag, docker.ImagePullOption{
			Registry: *registryConfig,
			Platform: params.Platform,
			OnProgress: func(progress map[string]*dockerTypes.PullProgress) {
				wsBuffer.BroadcastMessage(progress)
			},
		})
	} else {
		slog.Debug("image remote", "type", params.Type, "tag", imageNameDetail.Uri())
		// 推荐送镜像时保持原样
		// 自建仓库不需要添加 library
		// 即使推送 hub 镜像，library 命名空间属于官方空间，也不应该添加
		err = dockerClient.ImagePush(operationCtx, params.Tag, docker.ImagePushOption{
			Registry: *registryConfig,
			OnProgress: func(progress map[string]*dockerTypes.PullProgress) {
				wsBuffer.BroadcastMessage(progress)
			},
		})
	}
	if err != nil {
		if function.ErrorHasKeyword(err, "not found:", "repository does not exist") {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageImagePullTagNotFound, "tag", params.Tag), 500)
			return
		}
		if function.ErrorHasKeyword(err, "server gave HTTP response to HTTPS client") {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageImagePullServerHttp, "name", imageNameDetail.Registry), 500)
			return
		}
		if params.Type == "pull" {
			err = fmt.Errorf("image %s in Docker environment %s: %w", imageNameDetail.Uri(), dockerClient.Name, err)
		}
		self.JsonResponseWithError(http, err, 500)
		return
	}

	if params.Type == "pull" {
		newImage, err := dockerClient.Client.ImageInspect(operationCtx, params.Tag)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		if oldImage.ID == newImage.ID {
			self.JsonResponseWithoutError(http, gin.H{
				"proxyUrl": imageNameDetail.Registry,
				"tag":      params.Tag,
			})
			return
		}
		for _, tag := range oldImage.RepoTags {
			tagDetail := function.ImageTag(tag)
			tagDigest := tagDetail.Version[strings.LastIndex(tagDetail.Version, "-")+1:]
			if tagDetail.BaseName == "" || function.IsDockerObjectID(tagDigest) {
				continue
			}
			tagRegistry := (logic.Image{}).GetRegistryConfig(tagDetail.Registry)
			if slices.ContainsFunc(tagRegistry.ProxyAddress, func(address string) bool {
				return strings.EqualFold(function.RegistryReference(address), tagDetail.Registry)
			}) {
				continue
			}
			if err = dockerClient.Client.ImageTag(operationCtx, newImage.ID, tag); err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
		}
		if _, err = dockerClient.Client.ImageRemove(operationCtx, oldImage.ID, image.RemoveOptions{}); err != nil {
			slog.Debug("image pull kept old image", "imageId", oldImage.ID, "error", err)
			historyTag := *imageNameDetail
			historyTag.Version = strings.TrimPrefix(oldImage.ID, "sha256:")[:12]
			if err = dockerClient.Client.ImageTag(operationCtx, oldImage.ID, historyTag.Uri()); err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
		}
	}

	self.JsonResponseWithoutError(http, gin.H{
		"proxyUrl": imageNameDetail.Registry,
		"tag":      params.Tag,
	})
	return
}

func (self Image) TagDelete(http *gin.Context) {
	type ParamsValidate struct {
		Tag   string `json:"tag" binding:"required"`
		Force bool   `json:"force" binding:"omitempty"`
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
		Md5 string `json:"md5" binding:"required"`
		Tag string `json:"tag" binding:"required"`
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
	dockerClient, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	for _, address := range params.RegistryServerAddress {
		registryConfig := logic.Image{}.GetRegistryConfig(address)
		imagePushOption := docker.ImagePushOption{
			Registry: *registryConfig,
		}
		for _, md5 := range params.Md5 {
			imageDetail, err := dockerClient.Client.ImageInspect(dockerClient.Ctx, md5)
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
			for _, tag := range imageDetail.RepoTags {
				newImageName := function.ImageTag(tag)
				newImageName.Registry = address
				newImageName.Namespace = params.NewNamespace
				if !function.InArray(imageDetail.RepoTags, newImageName.Uri()) {
					err = dockerClient.Client.ImageTag(dockerClient.Ctx, tag, newImageName.Uri())
					if err != nil {
						self.JsonResponseWithError(http, err, 500)
						return
					}
				}
				err = dockerClient.ImagePush(dockerClient.Ctx, newImageName.Uri(), imagePushOption)
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
