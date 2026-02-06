package controller

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	http2 "net/http"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-units"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/registry"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type Image struct {
	controller.Abstract
}

func (self Image) ImportByContainerTar(http *gin.Context) {
	type ParamsValidate struct {
		Tar        string          `json:"tar" binding:"required"`
		Tag        []*function.Tag `json:"tag" binding:"required"`
		Registry   string          `json:"registry"`
		Cmd        string          `json:"cmd"`
		Entrypoint string          `json:"entrypoint"`
		WorkDir    string          `json:"workDir"`
		User       string          `json:"user"`
		Expose     []string        `json:"expose"`
		Env        []string        `json:"env"`
		Volume     []string        `json:"volume"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	tag := params.Tag[0].Name
	if params.Tag[0].Registry != "" {
		tag = params.Tag[0].Registry + "/" + tag
	}
	imageNameDetail := function.ImageTag(tag)
	imageInfo, err := docker.Sdk.Client.ImageInspect(docker.Sdk.Ctx, imageNameDetail.Uri())
	if err == nil && imageInfo.ID != "" {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonIdAlreadyExists, "name", imageNameDetail.Uri()), 500)
		return
	}
	containerTar, err := os.Open(storage.Local{}.GetSaveRealPath(params.Tar))
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer containerTar.Close()
	change := make([]string, 0)
	if params.Cmd != "" {
		change = append(change, "CMD "+params.Cmd)
	}
	if params.Entrypoint != "" {
		change = append(change, "ENTRYPOINT "+params.Entrypoint)
	}
	if params.WorkDir != "" {
		change = append(change, "WORKDIR "+params.WorkDir)
	}
	if params.User != "" {
		change = append(change, "USER "+params.User)
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
	}, imageNameDetail.Uri(), image.ImportOptions{
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
	_ = os.Remove(storage.Local{}.GetSaveRealPath(params.Tar))
	self.JsonSuccessResponse(http)
	return
}

func (self Image) ImportByImageTar(http *gin.Context) {
	type ParamsValidate struct {
		LocalUrl  []string `json:"localUrl"`
		RemoteUrl []string `json:"remoteUrl"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	wsBuffer := ws.NewProgressPip(ws.MessageTypeImageImport)
	defer wsBuffer.Close()

	importImageTag := make([]string, 0)
	wsBuffer.OnWrite = func(p string) error {
		slog.Debug("docker", "image build task", p)

		newReader := bufio.NewReader(bytes.NewReader([]byte(p)))
		for {
			line, _, err := newReader.ReadLine()
			if err == io.EOF {
				break
			}
			msg := types.BuildMessage{}
			if err = json.Unmarshal(line, &msg); err == nil {
				if msg.Stream != "" && strings.Contains(msg.Stream, "Loaded image:") {
					if _, after, exists := strings.Cut(msg.Stream, "Loaded image: "); exists {
						importImageTag = append(importImageTag, after)
					}
				}
				if msg.ErrorDetail.Message != "" {
					return errors.New(msg.ErrorDetail.Message)
				}
			} else {
				slog.Error("docker", "image build task", err, "data", p)
				return err
			}
		}
		return nil
	}

	tarPathList := make([]string, 0)
	if !function.IsEmptyArray(params.RemoteUrl) {
		for _, s := range params.RemoteUrl {
			err := func() error {
				response, err := http2.Get(s)
				if err != nil {
					return err
				}
				defer func() {
					_ = response.Body.Close()
				}()
				tempFile, err := storage.Local{}.CreateTempFile("")
				if err != nil {
					return err
				}
				defer func() {
					_ = tempFile.Close()
				}()
				_, _ = io.Copy(tempFile, response.Body)
				tarPathList = append(tarPathList, tempFile.Name())
				return nil
			}()
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
		}
	}

	if !function.IsEmptyArray(params.LocalUrl) {
		for _, s := range params.LocalUrl {
			tarPathList = append(tarPathList, storage.Local{}.GetSaveRealPath(s))
		}
	}

	for _, s := range tarPathList {
		err := func() error {
			imageTar, err := os.Open(s)
			if err != nil {
				return err
			}
			defer func() {
				_ = os.Remove(imageTar.Name())
			}()
			response, err := docker.Sdk.Client.ImageLoad(docker.Sdk.Ctx, imageTar, client.ImageLoadWithQuiet(false))
			if err != nil {
				return err
			}
			defer func() {
				if response.Body.Close() != nil {
					slog.Error("docker", "image import ", err)
				}
			}()
			_, err = io.Copy(wsBuffer, response.Body)
			return nil
		}()
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	_ = notice.Message{}.Info(".imageImport", "name", strings.Join(importImageTag, ", "))
	self.JsonResponseWithoutError(http, gin.H{
		"tag": importImageTag,
	})
	return
}

func (self Image) GetList(http *gin.Context) {
	type ParamsValidate struct {
		Tag string `json:"tag" binding:"omitempty"`
		Use int    `json:"use"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	var result []image.Summary
	imageList, err := docker.Sdk.Client.ImageList(docker.Sdk.Ctx, image.ListOptions{
		All:       false,
		Manifests: true,
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

		imageDetail, err := docker.Sdk.Client.ImageInspect(docker.Sdk.Ctx, summary.ID)
		if err == nil {
			if summary.Labels == nil {
				imageList[key].Labels = make(map[string]string)
			}
			imageList[key].Labels["com.dpanel.image.arch"] = imageDetail.Architecture
		}
	}

	if params.Tag != "" {
		result = function.PluckArrayWalk(imageList, func(item image.Summary) (image.Summary, bool) {
			for _, tag := range item.RepoTags {
				if strings.Contains(tag, params.Tag) {
					return item, true
				}
			}
			if strings.Contains(item.ID, params.Tag) {
				return item, true
			}
			return item, false
		})
	} else {
		result = imageList
	}

	if params.Use > 0 {
		result = function.PluckArrayWalk(result, func(i image.Summary) (image.Summary, bool) {
			if params.Use == 1 && i.Containers > 0 {
				return i, true
			}
			if params.Use == 2 && i.Containers == 0 {
				return i, true
			}
			return image.Summary{}, false
		})
	}

	self.JsonResponseWithoutError(http, gin.H{
		"list": result,
	})
	return
}

func (self Image) GetDetail(http *gin.Context) {
	type ParamsValidate struct {
		Md5       string `json:"md5" binding:"required"`
		ShowLayer bool   `json:"showLayer"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	var err error

	imageDetail, err := docker.Sdk.Client.ImageInspect(docker.Sdk.Ctx, params.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	layer := make([]image.HistoryResponseItem, 0)
	if params.ShowLayer {
		layer, err = docker.Sdk.Client.ImageHistory(docker.Sdk.Ctx, params.Md5)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	self.JsonResponseWithoutError(http, gin.H{
		"layer": layer,
		"info":  imageDetail,
	})
	return
}

func (self Image) Delete(http *gin.Context) {
	type ParamsValidate struct {
		Md5 []string `json:"md5" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	if !function.IsEmptyArray(params.Md5) {
		for _, sha := range params.Md5 {
			imageInfo, err := docker.Sdk.Client.ImageInspect(docker.Sdk.Ctx, sha)
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
			force := false
			// 如果镜像没有被使用，包含之个 tag 时需要增加 force 参数
			if len(imageInfo.RepoTags) > 1 {
				if list, err := docker.Sdk.Client.ContainerList(docker.Sdk.Ctx, container.ListOptions{
					All:     true,
					Filters: filters.NewArgs(filters.Arg("ancestor", sha)),
				}); err == nil && len(list) == 0 {
					force = true
				}
			}
			_, err = docker.Sdk.Client.ImageRemove(docker.Sdk.Ctx, sha, image.RemoveOptions{
				PruneChildren: true,
				Force:         force,
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

func (self Image) Prune(http *gin.Context) {
	type ParamsValidate struct {
		EnableUnuseTag bool `json:"enableUnuseTag"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	// 清理未使用的 tag 时，直接调用 Prune 处理
	// 只清理未使用镜像时，需要手动删除，避免 tag 被删除
	if params.EnableUnuseTag {
		filter := filters.NewArgs()
		filter.Add("dangling", "0")
		res, err := docker.Sdk.Client.ImagesPrune(docker.Sdk.Ctx, filter)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		_ = notice.Message{}.Info(".imagePrune", "size", units.HumanSize(float64(res.SpaceReclaimed)), "count", fmt.Sprintf("%d", len(res.ImagesDeleted)))
	} else {
		var deleteImageSpaceReclaimed int64 = 0
		deleteImageTotal := 0
		useImageList := make([]string, 0)
		if containerList, err := docker.Sdk.Client.ContainerList(docker.Sdk.Ctx, container.ListOptions{
			All: true,
		}); err == nil {
			useImageList = function.PluckArrayWalk(containerList, func(item container.Summary) (string, bool) {
				return item.ImageID, true
			})
		}
		if imageList, err := docker.Sdk.Client.ImageList(docker.Sdk.Ctx, image.ListOptions{
			All: true,
		}); err == nil {
			for _, item := range imageList {
				if !function.InArray(useImageList, item.ID) {
					deleteImageSpaceReclaimed += item.Size
					deleteImageTotal += 1
					// 当 tag 没有的时候把 id 也附加上
					item.RepoTags = append(item.RepoTags, item.ID)
					for _, tag := range item.RepoTags {
						_, err = docker.Sdk.Client.ImageRemove(docker.Sdk.Ctx, tag, image.RemoveOptions{
							PruneChildren: true,
						})
						if err != nil {
							slog.Debug("image prune image remove", "error", err)
						}
					}
				}
			}
		}
		_ = notice.Message{}.Info(".imagePrune", "size", units.HumanSize(float64(deleteImageSpaceReclaimed)), "count", fmt.Sprintf("%d", deleteImageTotal))
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Image) Export(http *gin.Context) {
	type ParamsValidate struct {
		Md5                []string `json:"md5" binding:"required"`
		EnableExportToPath bool     `json:"enableExportToPath"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	out, err := docker.Sdk.Client.ImageSave(docker.Sdk.Ctx, params.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		err = out.Close()
		if err != nil {
			slog.Debug("image export close", "error", err)
		}
	}()

	names := function.PluckArrayWalk(params.Md5, func(i string) (string, bool) {
		imageDetail := function.ImageTag(i)
		return strings.ReplaceAll(strings.ReplaceAll(imageDetail.BaseName, "-", "_"), "/", "_"), true
	})
	fileName := fmt.Sprintf("export/image/%s-%s.tar", strings.Join(names, "-"), time.Now().Format(define.DateYmdHis))
	if params.EnableExportToPath {
		file, err := storage.Local{}.CreateTempFile(fileName)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		defer func() {
			_ = file.Close()
		}()
		_, err = io.Copy(file, out)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		_ = notice.Message{}.Info(define.InfoMessageCommonExportInPath, "path", file.Name())
	} else {
		http.Header("Content-Type", "application/tar")
		http.Header("Content-Disposition", "attachment; filename="+fileName)
		http.DataFromReader(200, -1, "application/tar", out, nil)
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Image) CheckUpgrade(http *gin.Context) {
	type ParamsValidate struct {
		Tag       string `json:"tag" binding:"required"`
		Md5       string `json:"md5" binding:"required"`
		CacheTime int    `json:"cacheTime"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	imageInfo, err := docker.Sdk.Client.ImageInspect(docker.Sdk.Ctx, params.Md5)
	// 如果本地 digest 为空，则不检测
	if err != nil || function.IsEmptyArray(imageInfo.RepoDigests) {
		self.JsonResponseWithoutError(http, gin.H{
			"upgrade":     false,
			"digest":      "",
			"digestLocal": imageInfo.RepoDigests,
			"error":       err,
		})
		return
	}

	if params.CacheTime > 0 {
		if v, ok := storage.Cache.Get(fmt.Sprintf(storage.CacheKeyImageDigest, params.Md5)); ok {
			self.JsonResponseWithoutError(http, v)
			return
		}
	}

	result := gin.H{
		"upgrade":     false,
		"digest":      "",
		"digestLocal": imageInfo.RepoDigests,
		"error":       "",
	}

	imageNameDetail := function.ImageTag(params.Tag)
	registryConfig := logic.Image{}.GetRegistryConfig(imageNameDetail.Registry)

	option := make([]registry.Option, 0)
	option = append(option, registry.WithCredentialsWithBasic(registryConfig.Config.Username, registryConfig.Config.Password))
	option = append(option, registry.WithAddress(registryConfig.Address...))
	reg := registry.New(option...)
	if ok, desc, err := reg.Client().ManifestExist(imageNameDetail.BaseName, imageNameDetail.Version); err == nil && ok {
		result["digest"] = desc.Digest.String()
		if !function.InArrayWalk(imageInfo.RepoDigests, func(i string) bool {
			return strings.HasSuffix(i, desc.Digest.String())
		}) {
			result["upgrade"] = true
		}
	} else if err != nil {
		result["error"] = err.Error()
	}

	storage.Cache.Set(fmt.Sprintf(storage.CacheKeyImageDigest, params.Md5), result, time.Duration(params.CacheTime)*time.Second)
	self.JsonResponseWithoutError(http, result)
	return
}
