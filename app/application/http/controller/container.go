package controller

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-units"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/gin-gonic/gin"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

type Container struct {
	controller.Abstract
}

func (self Container) Status(http *gin.Context) {
	type ParamsValidate struct {
		Md5     string `form:"md5" binding:"required"`
		Operate string `form:"operate" binding:"required,oneof=start stop restart pause unpause"`
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
		Md5       string `json:"md5"`
		SiteTitle string `json:"siteTitle"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	list := make([]types.Container, 0)
	filter := filters.NewArgs()
	if params.Md5 != "" {
		filter.Add("id", params.Md5)
	}
	list, err := docker.Sdk.Client.ContainerList(docker.Sdk.Ctx, container.ListOptions{
		All:     true,
		Latest:  true,
		Filters: filter,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	if function.IsEmptyArray(list) {
		self.JsonResponseWithoutError(http, gin.H{
			"list": make([]types.Container, 0),
		})
		return
	}

	searchContainerIds := make([]string, 0)
	if params.SiteTitle != "" {
		searchSiteList, _ := dao.Site.Where(dao.Site.SiteTitle.Like("%" + params.SiteTitle + "%")).Find()
		for _, item := range searchSiteList {
			if item.ContainerInfo.ID != "" {
				searchContainerIds = append(searchContainerIds, item.ContainerInfo.ID)
			}
		}
	}

	var result []types.Container

	if params.SiteTitle != "" {
		result = make([]types.Container, 0)
		for _, item := range list {
			if function.InArray(searchContainerIds, item.ID) {
				result = append(result, item)
				continue
			}
			for _, name := range item.Names {
				if strings.Contains(name, params.SiteTitle) {
					result = append(result, item)
					break
				}
			}
		}
	} else {
		result = list
	}

	var md5List []driver.Valuer
	var nameList []string
	for index, item := range result {
		md5List = append(md5List, &accessor.SiteContainerInfoOption{
			ID: item.ID,
		})
		nameList = append(nameList, item.Names...)
		// 如果是直接绑定到宿主机网络，端口号不会显示到容器详情中
		// 需要通过镜像允许再次获取下
		if item.HostConfig.NetworkMode == "host" {
			imageInfo, _, err := docker.Sdk.Client.ImageInspectWithRaw(docker.Sdk.Ctx, item.ImageID)
			if err == nil {
				ports := []types.Port{}
				for port, _ := range imageInfo.Config.ExposedPorts {
					portInt, _ := strconv.Atoi(port.Port())
					ports = append(ports, types.Port{
						IP:          "0.0.0.0",
						PublicPort:  uint16(portInt),
						PrivatePort: uint16(portInt),
						Type:        port.Proto(),
					})
				}
				result[index].Ports = ports
			}
		}
	}

	query := dao.Site.Where(dao.Site.ContainerInfo.In(md5List...))
	siteList, _ := query.Find()

	domainList, _ := dao.SiteDomain.Where(dao.SiteDomain.ContainerID.In(nameList...)).Find()
	self.JsonResponseWithoutError(http, gin.H{
		"list":       result,
		"siteList":   siteList,
		"domainList": domainList,
	})
	return
}

func (self Container) GetDetail(http *gin.Context) {
	type ParamsValidate struct {
		Md5 string `form:"md5" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	detail, err := docker.Sdk.ContainerInfo(params.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"info": detail,
	})
	return
}

func (self Container) Update(http *gin.Context) {
	type ParamsValidate struct {
		Md5     string `json:"md5" binding:"required"`
		Restart string `json:"restart" binding:"omitempty,oneof=no on-failure unless-stopped always"`
		Name    string `json:"name"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if params.Restart != "" {
		restartPolicy := container.RestartPolicy{
			Name: docker.GetRestartPolicyByString(params.Restart),
		}
		if params.Restart == "on-failure" {
			restartPolicy.MaximumRetryCount = 5
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
	self.JsonSuccessResponse(http)
	return
}

func (self Container) Upgrade(http *gin.Context) {
	type ParamsValidate struct {
		Md5       string `json:"md5" binding:"required"`
		ImageTag  string `json:"imageTag"`
		EnableBak bool   `json:"enableBak"`
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
	bakTime := time.Now().Format(function.YmdHis)

	// 更新容器时可以更改镜像 tag
	if params.ImageTag != "" {
		containerInfo.Image = params.ImageTag
	}

	imageInfo, _, err := docker.Sdk.Client.ImageInspectWithRaw(docker.Sdk.Ctx, containerInfo.Config.Image)
	// 如果旧的容器使用的镜像和重新拉取的镜像一致则不升级
	// 多平台下的其它平台镜像推送后，也会提示更新
	// 不一定就是本平台镜像有更新
	oldContainerImageId := containerInfo.Image
	if containerInfo.Image == imageInfo.ID {
		//self.JsonResponseWithoutError(http, gin.H{
		//	"containerId": containerInfo.ID,
		//})
		//return
	}

	// 成功的创建一个新的容器后再对旧的进停止或是删除操作
	_ = notice.Message{}.Info("containerCreate", containerInfo.Name)
	newContainerName := fmt.Sprintf("%s-copy-%s", containerInfo.Name, bakTime)

	out, err := docker.Sdk.Client.ContainerCreate(docker.Sdk.Ctx, containerInfo.Config, containerInfo.HostConfig, &network.NetworkingConfig{
		EndpointsConfig: containerInfo.NetworkSettings.Networks,
	}, &v1.Platform{}, newContainerName)

	if err != nil {
		errRemove := docker.Sdk.Client.ContainerRemove(docker.Sdk.Ctx, newContainerName, container.RemoveOptions{})
		self.JsonResponseWithError(http, errors.Join(err, errRemove), 500)
		return
	}

	if containerInfo.State.Running {
		err = docker.Sdk.Client.ContainerStop(docker.Sdk.Ctx, containerInfo.Name, container.StopOptions{})
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	bakContainerName := fmt.Sprintf("%s-bak-%s", containerInfo.Name, bakTime)
	bakImageName := fmt.Sprintf("%s-bak-%s", containerInfo.Config.Image, bakTime)

	// 未备份旧容器，需要先删除，否则名称会冲突
	if params.EnableBak {
		_ = notice.Message{}.Info("containerBackup", "name", containerInfo.Name)

		// 备份旧容器
		err = docker.Sdk.Client.ContainerRename(
			docker.Sdk.Ctx,
			containerInfo.Name,
			bakContainerName,
		)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}

		if oldContainerImageId != imageInfo.ID {
			// 备份旧镜像
			err = docker.Sdk.Client.ImageTag(
				docker.Sdk.Ctx,
				containerInfo.Image,
				bakImageName,
			)
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
		}
	} else {
		_ = notice.Message{}.Info("containerRemove", containerInfo.Name)

		err = docker.Sdk.Client.ContainerRemove(docker.Sdk.Ctx, containerInfo.Name, container.RemoveOptions{})
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		_, err = docker.Sdk.Client.ImageRemove(docker.Sdk.Ctx, containerInfo.Image, image.RemoveOptions{})
		if err != nil {
			slog.Debug("container upgrade delete image", "error", err.Error())
		}
	}

	err = docker.Sdk.Client.ContainerRename(
		docker.Sdk.Ctx,
		newContainerName,
		containerInfo.Name,
	)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	if siteRow, _ := dao.Site.Where(dao.Site.ContainerInfo.Eq(&accessor.SiteContainerInfoOption{
		ID: params.Md5,
	})).First(); siteRow != nil {
		siteRow.ContainerInfo.ID = out.ID
		_, _ = dao.Site.Updates(siteRow)
	}

	err = docker.Sdk.Client.ContainerStart(docker.Sdk.Ctx, containerInfo.Name, container.StartOptions{})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
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
	_ = notice.Message{}.Info("containerPrune", "count", fmt.Sprintf("%d", len(info.ContainersDeleted)), "size", units.HumanSize(float64(info.SpaceReclaimed)))
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
	containerInfo, err := docker.Sdk.ContainerInfo(params.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	siteRow, _ := dao.Site.Where(dao.Site.ContainerInfo.Eq(&accessor.SiteContainerInfoOption{
		ID: params.Md5,
	})).First()

	if siteRow != nil && siteRow.SiteName != "" {
		// 删除网络
		// 获取该容器的网络，退出里面的容器
		networkInfo, err := docker.Sdk.Client.NetworkInspect(docker.Sdk.Ctx, siteRow.SiteName, network.InspectOptions{})
		if err == nil {
			for md5, _ := range networkInfo.Containers {
				err = docker.Sdk.Client.NetworkDisconnect(docker.Sdk.Ctx, siteRow.SiteName, md5, true)
				if err != nil {
					self.JsonResponseWithError(http, err, 500)
					return
				}

			}
			err = docker.Sdk.Client.NetworkRemove(docker.Sdk.Ctx, siteRow.SiteName)
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
		}
	}

	// 删除域名、配置、证书
	domainList, _ := dao.SiteDomain.Where(dao.SiteDomain.ContainerID.Eq(containerInfo.Name)).Find()
	for _, domain := range domainList {
		logic.Site{}.GetSiteNginxSetting(domain.ServerName).RemoveAll()
	}
	dao.SiteDomain.Where(dao.SiteDomain.ContainerID.Eq(containerInfo.ID)).Delete()

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
					slog.Debug("remove container volume", err.Error())
				}
			}
		}
	}

	if siteRow != nil {
		dao.Site.Where(dao.Site.ID.Eq(siteRow.ID)).Delete()
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
		Md5 string `json:"md5" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	out, err := docker.Sdk.Client.ContainerExport(docker.Sdk.Ctx, params.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer out.Close()

	data, err := io.ReadAll(out)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	http.Header("Content-Type", "application/tar")
	http.Header("Content-Disposition", "attachment; filename="+params.Md5+".tar")
	http.Data(200, "application/tar", data)
	return
}

func (self Container) Commit(http *gin.Context) {
	type ParamsValidate struct {
		Md5  string `json:"md5" binding:"required"`
		Name string `json:"name" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	response, err := docker.Sdk.Client.ContainerCommit(docker.Sdk.Ctx, params.Md5, container.CommitOptions{
		Reference: params.Name,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"md5": response.ID,
	})
	return
}
