package controller

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-units"
	"github.com/donknap/dpanel/app/application/logic"
	logic2 "github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/donknap/dpanel/common/types/event"
	"github.com/gin-gonic/gin"
	"github.com/mholt/archives"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/patrickmn/go-cache"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"gorm.io/datatypes"
	"gorm.io/gen"
	"gorm.io/gorm"
)

type Container struct {
	controller.Abstract
}

func (self Container) Status(http *gin.Context) {
	type ParamsValidate struct {
		Md5     string `json:"md5" binding:"required"`
		Operate string `json:"operate" binding:"required,oneof=start stop restart pause unpause"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	var err error
	switch params.Operate {
	case "restart":
		err = docker.Sdk.Client.ContainerRestart(docker.Sdk.Ctx,
			params.Md5,
			container.StopOptions{})
	case "stop":
		err = docker.Sdk.Client.ContainerStop(docker.Sdk.Ctx,
			params.Md5,
			container.StopOptions{})
	case "start":
		err = docker.Sdk.Client.ContainerStart(docker.Sdk.Ctx,
			params.Md5,
			container.StartOptions{})
	case "pause":
		err = docker.Sdk.Client.ContainerPause(docker.Sdk.Ctx,
			params.Md5)
	case "unpause":
		err = docker.Sdk.Client.ContainerUnpause(docker.Sdk.Ctx,
			params.Md5)
	}
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Container) GetList(http *gin.Context) {
	type ParamsValidate struct {
		SiteTitle string `json:"siteTitle"`
		Image     string `json:"image"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	sdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	list := make([]container.Summary, 0)
	list, err = sdk.ContainerList(sdk.Ctx, container.ListOptions{
		All:    true,
		Latest: true,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	if function.IsEmptyArray(list) {
		self.JsonResponseWithoutError(http, gin.H{
			"list":       make([]container.Summary, 0),
			"siteList":   make([]*entity.Site, 0),
			"domainList": make([]*entity.SiteDomain, 0),
		})
		return
	}

	containerID := params.SiteTitle
	isContainerID := function.IsDockerObjectID(containerID)
	imageID := strings.TrimPrefix(params.Image, "sha256:")
	isImageID := function.IsDockerObjectID(imageID)

	searchContainerIds := make([]string, 0)
	if params.SiteTitle != "" && !isContainerID {
		searchSiteList, _ := dao.Site.Where(dao.Site.SiteTitle.Like("%" + params.SiteTitle + "%")).Find()
		for _, item := range searchSiteList {
			if item.ContainerInfo.Id != "" {
				searchContainerIds = append(searchContainerIds, item.ContainerInfo.Id)
			}
		}
	}

	list = function.PluckArrayWalk(list, func(item container.Summary) (container.Summary, bool) {
		if v, ok := item.Labels[define.DPanelLabelContainerHidden]; ok && (v == "true" || v == "1") {
			return item, false
		}

		if function.IsEmptyArray(searchContainerIds) && params.Image == "" && params.SiteTitle == "" {
			return item, true
		}
		if params.Image != "" {
			if isImageID {
				itemImageID := strings.TrimPrefix(item.ImageID, "sha256:")
				if itemImageID == imageID || len(imageID) == 12 && strings.HasPrefix(itemImageID, imageID) {
					return item, true
				}
			} else if strings.Contains(item.Image, params.Image) {
				return item, true
			}
		}
		if params.SiteTitle != "" {
			if isContainerID {
				if item.ID == containerID || len(containerID) == 12 && strings.HasPrefix(item.ID, containerID) {
					return item, true
				}
			} else {
				if function.InArray(searchContainerIds, item.ID) {
					return item, true
				}
				for _, name := range item.Names {
					if strings.Contains(name, params.SiteTitle) {
						return item, true
					}
				}
			}
		}
		return item, false
	})

	containerName := make([]string, 0)
	for index, item := range list {
		containerName = append(containerName, item.Names...)
		containerInfo, err := sdk.Client.ContainerInspect(sdk.Ctx, item.ID)
		var inspectInfo *container.InspectResponse
		if err == nil {
			inspectInfo = &containerInfo
		}
		status := logic.Container{}.RuntimeStatus(logic.ContainerRuntimeItem{
			Summary: item,
			Inspect: inspectInfo,
		})
		if status.State != "" {
			list[index].State = status.State
		}
		if status.Message != "" {
			list[index].Status = status.Message
		}
		if inspectInfo != nil &&
			containerInfo.State != nil && containerInfo.State.Running &&
			containerInfo.HostConfig != nil && containerInfo.HostConfig.NetworkMode == network.NetworkHost &&
			containerInfo.Config != nil {
			for exposedPort := range containerInfo.Config.ExposedPorts {
				privatePort := uint16(exposedPort.Int())
				protocol := exposedPort.Proto()
				found := false
				for portIndex := range list[index].Ports {
					port := &list[index].Ports[portIndex]
					if port.PrivatePort != privatePort || !strings.EqualFold(port.Type, protocol) {
						continue
					}
					found = true
					if port.PublicPort == 0 {
						port.IP = "0.0.0.0"
						port.PublicPort = privatePort
					}
				}
				if found {
					continue
				}
				port := container.Port{
					IP:          "0.0.0.0",
					PrivatePort: privatePort,
					PublicPort:  privatePort,
					Type:        protocol,
				}
				list[index].Ports = append(list[index].Ports, port)
			}
		}
		sort.Slice(list[index].Ports, func(i, j int) bool {
			left, right := list[index].Ports[i], list[index].Ports[j]
			if left.PublicPort != right.PublicPort {
				if left.PublicPort == 0 {
					return false
				}
				if right.PublicPort == 0 {
					return true
				}
				return left.PublicPort < right.PublicPort
			}
			if left.PrivatePort != right.PrivatePort {
				return left.PrivatePort < right.PrivatePort
			}
			if left.Type != right.Type {
				return left.Type < right.Type
			}
			return left.IP < right.IP
		})
	}

	query := dao.Site.Where(dao.Site.SiteName.In(function.PluckArrayWalk(containerName, func(i string) (string, bool) {
		return strings.TrimLeft(i, "/"), true
	})...))
	siteList, _ := query.Find()

	sort.Slice(list, func(i, j int) bool {
		if function.IsEmptyArray(list[i].Names) || function.IsEmptyArray(list[j].Names) {
			return false
		}
		return list[i].Names[0] < list[j].Names[0]
	})

	domainList, _ := dao.SiteDomain.Where(dao.SiteDomain.ContainerID.In(containerName...)).Find()

	self.JsonResponseWithoutError(http, gin.H{
		"list":       list,
		"siteList":   siteList,
		"domainList": domainList,
	})
	return
}

func (self Container) GetDetail(http *gin.Context) {
	type ParamsValidate struct {
		Md5 string `json:"md5" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	detail, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, params.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	dpanelInfo := logic2.Setting{}.GetDPanelInfo()
	if docker.Sdk.DockerEnv.Default && dpanelInfo.ContainerInfo.ContainerJSONBase != nil && dpanelInfo.ContainerInfo.Name == detail.Name {
		detail.Config.Labels[define.DPanelLabelContainerDPanelSelf] = "true"
	}
	domain, _ := dao.SiteDomain.Where(dao.SiteDomain.ContainerID.In(detail.Name)).Find()
	self.JsonResponseWithoutError(http, gin.H{
		"info":   detail,
		"domain": domain,
	})
	return
}

func (self Container) Update(http *gin.Context) {
	type ParamsValidate struct {
		Md5     string               `json:"md5" binding:"required"`
		Restart *types.RestartPolicy `json:"restart"`
		Name    string               `json:"name"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if params.Restart != nil {
		restartPolicy := container.RestartPolicy{}
		if params.Restart.Name != "" {
			restartPolicy.Name = function.ParseRestartPolicy(params.Restart.Name)
		}
		if restartPolicy.Name == container.RestartPolicyOnFailure {
			restartPolicy.MaximumRetryCount = 5
		}
		if params.Restart.MaxAttempt > 0 {
			restartPolicy.Name = container.RestartPolicyOnFailure
			restartPolicy.MaximumRetryCount = params.Restart.MaxAttempt
		}
		_, err := docker.Sdk.Client.ContainerUpdate(docker.Sdk.Ctx, params.Md5, container.UpdateConfig{
			RestartPolicy: restartPolicy,
		})
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	if params.Name != "" {
		err := docker.Sdk.Client.ContainerRename(docker.Sdk.Ctx, params.Md5, params.Name)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	containerInfo, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, params.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	if siteRow, err := dao.Site.Where(gen.Cond(datatypes.JSONQuery("container_info").Equals(params.Md5, "Id"))...).First(); err == nil {
		siteRow.SiteName = strings.TrimLeft(params.Name, "/")
		siteRow.ContainerInfo = &accessor.SiteContainerInfoOption{
			Id:   params.Md5,
			Info: containerInfo,
		}
		_ = dao.Site.Save(siteRow)
	}

	self.JsonSuccessResponse(http)
	return
}

func (self Container) Copy(http *gin.Context) {
	type ParamsValidate struct {
		Md5              string `json:"md5" binding:"required"`
		CopyName         string `json:"copyName" binding:"required"`
		EnableRandomPort bool   `json:"enableRandomPort"`
		EnableStart      bool   `json:"enableStart"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	containerInfo, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, params.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	// 获取一个镜像 tag 是否存在，如果不存在则改用镜像 hash
	if _, err := docker.Sdk.Client.ImageInspect(docker.Sdk.Ctx, containerInfo.Config.Image); err != nil {
		containerInfo.Config.Image = containerInfo.Image
	}

	if _, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, params.CopyName); err == nil {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonIdAlreadyExists, "name", params.CopyName), 500)
		return
	}

	if params.EnableRandomPort && !function.IsEmptyMap(containerInfo.HostConfig.PortBindings) {
		for destPort, bindings := range containerInfo.HostConfig.PortBindings {
			if function.IsEmptyArray(bindings) {
				continue
			}
			for i := range bindings {
				containerInfo.HostConfig.PortBindings[destPort][i].HostPort = ""
			}
		}
	}
	out, err := docker.Sdk.Client.ContainerCreate(docker.Sdk.Ctx, containerInfo.Config, containerInfo.HostConfig, &network.NetworkingConfig{
		EndpointsConfig: containerInfo.NetworkSettings.Networks,
	}, &v1.Platform{}, params.CopyName)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	if params.EnableStart {
		err = docker.Sdk.Client.ContainerStart(docker.Sdk.Ctx, params.CopyName, container.StartOptions{})
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	self.JsonResponseWithoutError(http, gin.H{
		"containerId": out.ID,
	})
	return
}

func (self Container) Prune(http *gin.Context) {
	filter := filters.NewArgs()
	info, err := docker.Sdk.Client.ContainersPrune(docker.Sdk.Ctx, filter)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	_ = notice.Message{}.Info(".containerPrune", "count", fmt.Sprintf("%d", len(info.ContainersDeleted)), "size", units.HumanSize(float64(info.SpaceReclaimed)))
	self.JsonSuccessResponse(http)
	return
}

func (self Container) Delete(http *gin.Context) {
	type ParamsValidate struct {
		Md5          string `json:"md5" binding:"required"`
		DeleteImage  bool   `json:"deleteImage" binding:"omitempty"`
		DeleteVolume bool   `json:"deleteVolume" binding:"omitempty"`
		DeleteLink   bool   `json:"deleteLink" binding:"omitempty"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	var err error
	containerInfo, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, params.Md5)
	if err != nil {
		// 如果查找不到容器信息，可能是有错误的容器，强制删除
		err = docker.Sdk.Client.ContainerStop(docker.Sdk.Ctx, params.Md5, container.StopOptions{})
		if err != nil {
			slog.Warn("container delete not info container", "error", err)
		}
		err = docker.Sdk.Client.ContainerRemove(docker.Sdk.Ctx, params.Md5, container.RemoveOptions{
			Force: true,
		})
		if err != nil {
			slog.Warn("container delete not info container", "error", err)
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
			return
		}
		self.JsonResponseWithoutError(http, gin.H{
			"md5": params.Md5,
		})
		return
	}
	runOption, err := logic.Site{}.GetEnvOptionByContainer(params.Md5)
	if err != nil {
		slog.Warn("container delete create recycle", "error", err)
	}
	runOption.Command = ""
	runOption.Entrypoint = ""
	runOption.WorkDir = ""

	siteRow, _ := dao.Site.
		Where(gen.Cond(datatypes.JSONQuery("container_info").Equals(containerInfo.ID, "id"))...).
		Or(gen.Cond(datatypes.JSONQuery("container_info").Equals(containerInfo.ID, "Id"))...).
		First()
	// 创建回收站数据
	if siteRow == nil {
		siteRow = &entity.Site{
			SiteName: strings.TrimLeft(containerInfo.Name, "/"),
			Env:      &runOption,
			ContainerInfo: &accessor.SiteContainerInfoOption{
				Info: containerInfo,
				Id:   containerInfo.ID,
			},
			Status:     0,
			StatusStep: "",
			Message:    "",
			DeletedAt:  gorm.DeletedAt{},
		}
	} else {
		siteRow.ContainerInfo = &accessor.SiteContainerInfoOption{
			Info: containerInfo,
			Id:   containerInfo.ID,
		}
	}
	_ = dao.Site.Save(siteRow)

	// 如果存在 site 数据，则只保留最后一条
	_, _ = dao.Site.Unscoped().Where(gen.Cond(
		datatypes.JSONQuery("env").Equals(docker.Sdk.Name, "dockerEnvName"),
	)...).Where(dao.Site.SiteName.Eq(siteRow.SiteName)).Where(dao.Site.DeletedAt.IsNotNull()).Delete()

	// 删除域名、配置、证书
	domainList, _ := dao.SiteDomain.Where(dao.SiteDomain.ContainerID.Eq(containerInfo.Name)).Find()
	for _, domain := range domainList {
		err = os.Remove(storage.Local{}.GetNginxSettingFilePath(domain.Setting.VHostFilename()))
		if err != nil {
			slog.Warn("container delete domain", "error", err)
		}
	}

	_, err = dao.SiteDomain.Where(dao.SiteDomain.ContainerID.Eq(containerInfo.ID)).Delete()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	err = docker.Sdk.Client.ContainerStop(docker.Sdk.Ctx, containerInfo.ID, container.StopOptions{})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	err = docker.Sdk.Client.ContainerRemove(docker.Sdk.Ctx, containerInfo.ID, container.RemoveOptions{
		RemoveVolumes: params.DeleteVolume,
		RemoveLinks:   params.DeleteLink,
	})

	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	if params.DeleteImage {
		_, err = docker.Sdk.Client.ImageRemove(docker.Sdk.Ctx, containerInfo.Image, image.RemoveOptions{
			Force:         true,
			PruneChildren: true,
		})
	}

	if params.DeleteVolume {
		for _, item := range containerInfo.Mounts {
			if item.Type == mount.TypeVolume {
				err = docker.Sdk.Client.VolumeRemove(docker.Sdk.Ctx, item.Name, false)
				if err != nil {
					slog.Warn("remove container volume", "error", err.Error())
				}
			}
		}
	}

	facade.GetEvent().Publish(event.ContainerDeleteEvent, event.ContainerPayload{
		InspectInfo: &containerInfo,
		Ctx:         http,
	})

	if siteRow != nil {
		_, _ = dao.Site.Where(dao.Site.ID.Eq(siteRow.ID)).Delete()
		self.JsonResponseWithoutError(http, gin.H{
			"siteId": siteRow.ID,
			"md5":    params.Md5,
		})
	} else {
		self.JsonResponseWithoutError(http, gin.H{
			"md5": params.Md5,
		})
	}
	return
}

func (self Container) Export(http *gin.Context) {
	type ParamsValidate struct {
		Md5                  string `json:"md5"`
		EnableExportToPath   bool   `json:"enableExportToPath"`
		EnableExportCompress bool   `json:"enableExportCompress"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	containerInfo, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, params.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	out, err := docker.Sdk.Client.ContainerExport(docker.Sdk.Ctx, containerInfo.ID)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		_ = out.Close()
	}()
	fileName := strings.Trim(containerInfo.Name, "/") + "-" + time.Now().Format(define.DateYmdHis) + ".tar"
	if params.EnableExportCompress {
		fileName += ".zst"
	}
	exportSaveFile, err := storage.Local{}.CreateSaveFile("export/container/" + fileName)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		_ = exportSaveFile.Close()
	}()
	if params.EnableExportCompress {
		compression := archives.Zstd{}
		cw, err := compression.OpenWriter(exportSaveFile)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		defer func() {
			_ = cw.Close()
		}()
		_, err = io.Copy(cw, out)
	} else {
		_, err = io.Copy(exportSaveFile, out)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	if params.EnableExportToPath {
		self.JsonResponseWithoutError(http, gin.H{
			"saveUrl": exportSaveFile.Name(),
		})
		return
	}
	downloadUrl, err := logic2.Attach{}.PreDownload(exportSaveFile.Name(), cache.DefaultExpiration)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"saveUrl":     exportSaveFile.Name(),
		"downloadUrl": downloadUrl,
	})
}

func (self Container) Commit(http *gin.Context) {
	type ParamsValidate struct {
		Md5   string `json:"md5" binding:"required"`
		Name  string `json:"name"`
		Merge bool   `json:"merge"`
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
	containerInfo, err := dockerClient.Client.ContainerInspect(dockerClient.Ctx, params.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	imageName, err := dockerClient.ContainerCommit(dockerClient.Ctx, params.Md5, docker.ContainerCommitOption{
		Tag:   params.Name,
		Merge: params.Merge,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	noticeTitle := ".containerCommit"
	if params.Merge {
		noticeTitle = ".containerCommitMerge"
	}
	_ = notice.Message{}.Info(noticeTitle, "name", strings.TrimPrefix(containerInfo.Name, "/"))
	self.JsonResponseWithoutError(http, gin.H{
		"name": imageName,
	})
	return
}
