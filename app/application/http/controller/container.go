package controller

import (
	"fmt"
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
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/donknap/dpanel/common/types/event"
	"github.com/gin-gonic/gin"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"gorm.io/datatypes"
	"gorm.io/gen"
	"gorm.io/gorm"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
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
		Md5       string `json:"md5"`
		SiteTitle string `json:"siteTitle"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	list := make([]container.Summary, 0)
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
			"list": make([]container.Port, 0),
		})
		return
	}

	searchContainerIds := make([]string, 0)
	if params.SiteTitle != "" {
		searchSiteList, _ := dao.Site.Where(dao.Site.SiteTitle.Like("%" + params.SiteTitle + "%")).Find()
		for _, item := range searchSiteList {
			if item.ContainerInfo.Id != "" {
				searchContainerIds = append(searchContainerIds, item.ContainerInfo.Id)
			}
		}
	}

	result := make([]container.Summary, 0)
	if params.SiteTitle != "" {
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

	var containerName []string
	for index, item := range result {
		containerName = append(containerName, item.Names...)
		// 如果是直接绑定到宿主机网络或是 Macvlan，端口号不会显示到容器详情中
		// 需要通过获取镜像详情数据获取一下
		if function.IsEmptyArray(item.Ports) {
			if info, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, item.ID); err == nil && info.HostConfig != nil && !function.IsEmptyMap(info.HostConfig.PortBindings) {
				ports := make([]container.Port, 0)
				for port, bindings := range info.HostConfig.PortBindings {
					for _, binding := range bindings {
						hostPort, _ := strconv.Atoi(binding.HostPort)
						if binding.HostIP == "" {
							binding.HostIP = "0.0.0.0"
						}
						ports = append(ports, container.Port{
							IP:          binding.HostIP,
							PublicPort:  uint16(hostPort),
							PrivatePort: uint16(port.Int()),
							Type:        port.Proto(),
						})
					}
				}
				result[index].Ports = ports
			}
		}
	}

	query := dao.Site.Where(dao.Site.SiteName.In(function.PluckArrayWalk(containerName, func(i string) (string, bool) {
		return strings.TrimLeft(i, "/"), true
	})...))
	siteList, _ := query.Find()

	sort.Slice(result, func(i, j int) bool {
		return result[i].Names[0] < result[j].Names[0]
	})

	domainList, _ := dao.SiteDomain.Where(dao.SiteDomain.ContainerID.In(containerName...)).Find()

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

	detail, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, params.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	ignore := accessor.ContainerCheckIgnoreUpgrade{}
	logic2.Setting{}.GetByKey(logic2.SettingGroupSetting, logic2.SettingGroupSettingCheckContainerIgnore, &ignore)

	domain, _ := dao.SiteDomain.Where(dao.SiteDomain.ContainerID.In(detail.Name)).Find()
	self.JsonResponseWithoutError(http, gin.H{
		"info":   detail,
		"ignore": ignore,
		"domain": domain,
	})
	return
}

func (self Container) Update(http *gin.Context) {
	type ParamsValidate struct {
		Md5     string                `json:"md5" binding:"required"`
		Restart *docker.RestartPolicy `json:"restart"`
		Name    string                `json:"name"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if params.Restart != nil {
		restartPolicy := container.RestartPolicy{}
		if params.Restart.Name != "" {
			restartPolicy.Name = docker.GetRestartPolicyByString(params.Restart.Name)
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
	_ = docker.Sdk.Client.ContainerStart(docker.Sdk.Ctx, params.CopyName, container.StartOptions{})

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
		self.JsonResponseWithError(http, err, 500)
		return
	}
	runOption, err := logic.Site{}.GetEnvOptionByContainer(params.Md5)
	if err != nil {
		slog.Warn("container delete create recycle", "error", err)
	}
	runOption.Command = ""
	runOption.Entrypoint = ""
	runOption.WorkDir = ""

	siteRow, _ := dao.Site.Where(gen.Cond(datatypes.JSONQuery("container_info").Equals(containerInfo.ID, "Id"))...).First()
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
	// 删除域名、配置、证书
	domainList, _ := dao.SiteDomain.Where(dao.SiteDomain.ContainerID.Eq(containerInfo.Name)).Find()
	for _, domain := range domainList {
		err = os.Remove(filepath.Join(storage.Local{}.GetNginxSettingPath(), fmt.Sprintf(logic.VhostFileName, domain.ServerName)))
		if err != nil {
			slog.Debug("container delete domain", "error", err)
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
					slog.Debug("remove container volume", err.Error())
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
	defer func() {
		_ = out.Close()
	}()

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
