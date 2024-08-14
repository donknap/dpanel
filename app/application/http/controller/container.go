package controller

import (
	"database/sql/driver"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"io"
	"log/slog"
	"strconv"
	"strings"
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
		Tag       string `json:"tag"`
		Md5       string `json:"md5"`
		SiteTitle string `json:"siteTitle"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	var list []types.Container
	filter := filters.NewArgs()
	if params.Tag != "" {
		filter.Add("name", params.Tag)
	}
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

	var md5List []driver.Valuer
	var nameList []string
	for index, item := range list {
		md5List = append(md5List, &accessor.SiteContainerInfoOption{
			ID: item.ID,
		})
		nameList = append(nameList, item.Names...)
		// 如果是直接绑定到宿主机网络，端口号不会显示到容器详情中
		// 需要通过镜像允许再次获取下
		if item.HostConfig.NetworkMode == "host" {
			imageInfo, _, err := docker.Sdk.Client.ImageInspectWithRaw(docker.Sdk.Ctx, item.ImageID)
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
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
			list[index].Ports = ports
		}
	}

	query := dao.Site.Where(dao.Site.ContainerInfo.In(md5List...))

	if params.SiteTitle != "" {
		query = query.Where(dao.Site.SiteTitle.Like("%" + params.SiteTitle + "%"))
	}
	siteList, _ := query.Find()

	domainList, _ := dao.SiteDomain.Where(dao.SiteDomain.ContainerID.In(nameList...)).Find()
	self.JsonResponseWithoutError(http, gin.H{
		"list":       list,
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
			Name: docker.Sdk.GetRestartPolicyByString(params.Restart),
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

func (self Container) Prune(http *gin.Context) {
	filter := filters.NewArgs()
	docker.Sdk.Client.ContainersPrune(docker.Sdk.Ctx, filter)
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

	if siteRow != nil {
		dao.Site.Where(dao.Site.ID.Eq(siteRow.ID)).Delete()
		if params.DeleteVolume {
			volumeList, _ := docker.Sdk.Client.VolumeList(docker.Sdk.Ctx, volume.ListOptions{})
			for _, volueItem := range volumeList.Volumes {
				if strings.HasPrefix(volueItem.Name, siteRow.SiteName) {
					err = docker.Sdk.Client.VolumeRemove(docker.Sdk.Ctx, volueItem.Name, false)
					if err != nil {
						slog.Debug("remove container volume", err.Error())
					}
				}
			}
		}

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
