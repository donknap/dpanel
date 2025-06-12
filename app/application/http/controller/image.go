package controller

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-units"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/registry"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"io"
	"log/slog"
	http2 "net/http"
	"os"
	"strings"
	"time"
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
	imageNameDetail := registry.GetImageTagDetail(params.Tag)
	if params.Registry != "" {
		imageNameDetail.Registry = params.Registry
	}
	imageInfo, err := docker.Sdk.Client.ImageInspect(docker.Sdk.Ctx, imageNameDetail.Uri())
	if err == nil && imageInfo.ID != "" {
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
	}
	if params.WorkDir != "" {
		change = append(change, "WORKDIR "+params.WorkDir)
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
	_ = os.Remove(storage.Local{}.GetRealPath(params.Tar))
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
			msg := docker.BuildMessage{}
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
			tarPathList = append(tarPathList, storage.Local{}.GetRealPath(s))
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

	notice.Message{}.Info(".imageImport", "name", strings.Join(importImageTag, ", "))
	self.JsonResponseWithoutError(http, gin.H{
		"tag": importImageTag,
	})
	return
}

func (self Image) CreateByDockerfile(http *gin.Context) {
	params := logic.BuildImageOption{}
	if !self.Validate(http, &params) {
		return
	}
	if params.BuildDockerfileContent == "" && params.BuildZip == "" && params.BuildGit == "" {
		self.JsonResponseWithError(http, errors.New("至少需要指定 Dockerfile、Zip 包或是 Git 地址"), 500)
		return
	}
	if params.BuildZip != "" && params.BuildGit != "" {
		self.JsonResponseWithError(http, errors.New("zip 包和 git 地址只需要只定一项"), 500)
		return
	}
	imageNameDetail := registry.GetImageTagDetail(params.Tag)
	if params.Registry != "" {
		imageNameDetail.Registry = params.Registry
	}
	params.Tag = imageNameDetail.Uri()

	if params.BuildZip != "" {
		path := storage.Local{}.GetRealPath(params.BuildZip)
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			self.JsonResponseWithError(http, errors.New("请先上传压缩包"), 500)
			return
		}
		params.BuildZip = path
	}

	imageNew := &entity.Image{
		Tag:   imageNameDetail.Uri(),
		Title: params.Title,
		Setting: &accessor.ImageSettingOption{
			Registry:            params.Registry,
			BuildGit:            params.BuildGit,
			BuildDockerfile:     params.BuildDockerfileContent,
			BuildRoot:           params.BuildDockerfileRoot,
			BuildDockerfileName: params.BuildDockerfileName,
			BuildArgs:           params.BuildArgs,
			Platform:            &params.Platform,
		},
		BuildType: params.BuildType,
		Status:    docker.ImageBuildStatusStop,
		Message:   "",
	}
	if imageRow, _ := dao.Image.Where(dao.Image.ID.Eq(params.Id)).First(); imageRow != nil {
		imageNew.ID = imageRow.ID
	}
	_ = dao.Image.Save(imageNew)

	params.MessageId = fmt.Sprintf(ws.MessageTypeImageBuild, imageNew.ID)
	log, err := logic.DockerTask{}.ImageBuild(&params)
	if err != nil {
		imageNew.Status = docker.ImageBuildStatusError
		imageNew.Message = log + "\n" + err.Error()
		_ = dao.Image.Save(imageNew)
		self.JsonResponseWithError(http, err, 500)
		return
	}
	imageNew.Status = docker.ImageBuildStatusSuccess
	imageNew.Message = log
	imageNew.ImageInfo = &accessor.ImageInfoOption{
		Id: imageNameDetail.Uri(),
	}
	_ = dao.Image.Save(imageNew)
	self.JsonResponseWithoutError(http, gin.H{
		"imageId": imageNew.ID,
	})
	return
}

func (self Image) GetList(http *gin.Context) {
	type ParamsValidate struct {
		Tag   string `form:"tag" binding:"omitempty"`
		Title string `json:"title"`
		Use   int    `json:"use"`
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

		imageDetail, err := docker.Sdk.Client.ImageInspect(docker.Sdk.Ctx, summary.ID)
		if err == nil {
			if summary.Labels == nil {
				imageList[key].Labels = make(map[string]string)
			}
			imageList[key].Labels["com.dpanel.image.arch"] = imageDetail.Architecture
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

	titleList := make(map[string]string)
	imageDbList, err := dao.Image.Find()
	if err == nil {
		for _, item := range imageDbList {
			titleList[item.Tag] = item.Title
		}
	}
	self.JsonResponseWithoutError(http, gin.H{
		"list":  result,
		"title": titleList,
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
				Force:         true,
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
	res, err := docker.Sdk.Client.ImagesPrune(docker.Sdk.Ctx, filter)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	_ = notice.Message{}.Info(".imagePrune", "size", units.HumanSize(float64(res.SpaceReclaimed)), "count", fmt.Sprintf("%d", len(res.ImagesDeleted)))
	self.JsonSuccessResponse(http)
	return
}

func (self Image) BuildPrune(http *gin.Context) {
	res, err := docker.Sdk.Client.BuildCachePrune(docker.Sdk.Ctx, types.BuildCachePruneOptions{
		All: true,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	_ = notice.Message{}.Info(".imageBuildPrune", "size", units.HumanSize(float64(res.SpaceReclaimed)))
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

	var writer io.Writer
	var file *os.File

	if params.EnableExportToPath {
		names := function.PluckArrayWalk(params.Md5, func(i string) (string, bool) {
			imageDetail := registry.GetImageTagDetail(i)
			return strings.ReplaceAll(strings.ReplaceAll(imageDetail.BaseName, "-", "_"), "/", "_"), true
		})
		file, err = storage.Local{}.CreateTempFile(fmt.Sprintf("export/image/%s-%s.tar", strings.Join(names, "-"), time.Now().Format(function.YmdHis)))
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		defer func() {
			_ = file.Close()
		}()
		writer = file
		_ = notice.Message{}.Info(".imageExportInPath", "path", file.Name())
	} else {
		writer = http.Writer
	}
	_, err = io.Copy(writer, out)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Image) UpdateTitle(http *gin.Context) {
	type ParamsValidate struct {
		Tag   string `json:"tag" binding:"required"`
		Title string `json:"title" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	imageBuildRow, _ := dao.Image.Where(dao.Image.Tag.Eq(params.Tag)).First()
	if imageBuildRow != nil {
		dao.Image.Where(dao.Image.Tag.Eq(params.Tag)).Updates(&entity.Image{
			Title: params.Title,
		})
	} else {
		_ = dao.Image.Create(&entity.Image{
			Title:     params.Title,
			Tag:       params.Tag,
			BuildType: "pull",
		})
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
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	// 如果本地 digest 为空，则不检测
	if function.IsEmptyArray(imageInfo.RepoDigests) {
		_ = notice.Message{}.Info(".imageCheckUpgradeImageNotDigest")
		self.JsonResponseWithoutError(http, gin.H{
			"upgrade":     false,
			"digest":      "",
			"digestLocal": imageInfo.RepoDigests,
		})
		return
	}

	digest := ""
	upgrade := false

	imageNameDetail := registry.GetImageTagDetail(params.Tag)
	registryConfig := logic.Image{}.GetRegistryConfig(imageNameDetail.Uri())

	for _, s := range registryConfig.Proxy {
		option := make([]registry.Option, 0)
		if params.CacheTime > 0 {
			option = append(option, registry.WithRequestCacheTime(time.Second*time.Duration(params.CacheTime)))
		}
		option = append(option, registry.WithCredentialsString(registryConfig.GetRegistryAuthString()))
		option = append(option, registry.WithRegistryHost(s))
		reg := registry.New(option...)
		if digest, err = reg.Repository.GetImageDigest(params.Tag); err == nil {
			slog.Debug("image check upgrade", "remote digest", fmt.Sprintf("%s@%s", params.Tag, digest), "local digest", imageInfo.RepoDigests)
			if !function.InArrayWalk(imageInfo.RepoDigests, func(i string) bool {
				return strings.HasSuffix(i, digest)
			}) {
				upgrade = true
			}
			break
		} else {
			slog.Debug("image check upgrade", "err", err.Error())
		}
	}
	result := gin.H{
		"upgrade":     upgrade,
		"digest":      digest,
		"digestLocal": imageInfo.RepoDigests,
		"error":       "",
	}
	if err != nil {
		result["error"] = err.Error()
	}
	self.JsonResponseWithoutError(http, result)
	return
}
