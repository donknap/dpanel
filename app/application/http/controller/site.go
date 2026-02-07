package controller

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/donknap/dpanel/app/application/logic/task"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/donknap/dpanel/common/types/event"
	"github.com/gin-gonic/gin"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"gorm.io/datatypes"
	"gorm.io/gen"
	"gorm.io/gorm"
)

type Site struct {
	controller.Abstract
}

func (self Site) CreateByImage(http *gin.Context) {
	type ParamsValidate struct {
		SiteTitle   string `json:"siteTitle"`
		SiteName    string `json:"siteName" binding:"required"`
		ImageName   string `json:"imageName" binding:"required"`
		ContainerId string `json:"id"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	buildParams := accessor.SiteEnvOption{}
	if !self.Validate(http, &buildParams) {
		return
	}

	checkIpInSubnet := make([][2]string, 0)
	if buildParams.IpV4 != nil {
		checkIpInSubnet = append(checkIpInSubnet, [2]string{
			buildParams.IpV4.Address, buildParams.IpV4.Subnet,
		}, [2]string{
			buildParams.IpV4.Gateway, buildParams.IpV4.Subnet,
		})
	}
	if buildParams.IpV6 != nil {
		checkIpInSubnet = append(checkIpInSubnet, [2]string{
			buildParams.IpV6.Address, buildParams.IpV6.Subnet,
		}, [2]string{
			buildParams.IpV6.Gateway, buildParams.IpV6.Subnet,
		})
	}
	for _, item := range checkIpInSubnet {
		if item[0] == "" {
			continue
		}
		_, err := function.IpInSubnet(item[0], item[1])
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	var siteRow *entity.Site

	// 重新部署，先删掉之前的容器数据
	// 删除数据时应该查找对应的Id 和 名称都相同的，避免多环境下名称一致导致删除错误
	// 删除容器时，先把记录设置为软删除，部署失败后在回收站中可以查看
	if params.ContainerId != "" {
		_, _ = dao.Site.
			Where(gen.Cond(datatypes.JSONQuery("container_info").Equals(params.ContainerId, "Id"))...).
			Delete()
		deleteQuery := dao.Site.Unscoped().Order(dao.Site.ID.Desc()).Where(gen.Cond(
			datatypes.JSONQuery("env").Equals(docker.Sdk.Name, "dockerEnvName"),
		)...).Where(dao.Site.SiteName.Eq(params.SiteName)).Where(dao.Site.DeletedAt.IsNotNull())
		deleteIds := make([]int32, 0)
		if err := deleteQuery.Limit(5).Pluck(dao.Site.ID, &deleteIds); err == nil {
			_, _ = deleteQuery.Where(dao.Site.ID.NotIn(deleteIds...)).Delete()
		}
	}

	// 创建前先查找一下当前环境下是否有容器
	// 如果有，则查询数据中是否有记录，有就表示是更新，否则是创建
	if containerInfo, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, params.SiteName); err == nil {
		siteRow, _ = dao.Site.Where(gen.Cond(datatypes.JSONQuery("container_info").Equals(containerInfo.ID, "Id"))...).First()
		if siteRow != nil && params.ContainerId == "" {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonIdAlreadyExists, "name", params.SiteName), 500)
			return
		}
	}

	imageInfo, err := docker.Sdk.Client.ImageInspect(docker.Sdk.Ctx, params.ImageName)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	buildParams.ImageId = imageInfo.ID

	for i, volume := range buildParams.Volumes {
		if volume.Host == "" {
			buildParams.Volumes[i].Host = fmt.Sprintf("dpanel.%s.%s",
				params.SiteName,
				strings.Join(strings.Split(volume.Dest, "/"), "-"),
			)
		}
	}

	buildParams.DockerEnvName = docker.Sdk.Name

	siteRow = &entity.Site{
		SiteName:      params.SiteName,
		SiteTitle:     params.SiteTitle,
		Env:           &buildParams,
		Status:        define.DockerImageBuildStatusStop,
		ContainerInfo: &accessor.SiteContainerInfoOption{},
	}
	// 获取一下当前是否有容器，出错后，还可以获取到最后一次成功的配置
	if detail, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, params.SiteName); err == nil {
		siteRow.ContainerInfo = &accessor.SiteContainerInfoOption{
			Id:   detail.ID,
			Info: detail,
		}
	}

	err = dao.Site.Create(siteRow)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	runTaskRow := &task.CreateContainerOption{
		SiteName:    siteRow.SiteName,
		SiteId:      siteRow.ID,
		BuildParams: &buildParams,
		ContainerId: params.ContainerId,
	}
	containerId, err := task.Docker{}.ContainerCreate(runTaskRow)
	if err != nil {
		if containerId != "" {
			// 如果容器在启动时发生错误，需要先删除掉
			_, err1 := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, containerId)
			if err1 == nil {
				_ = docker.Sdk.Client.ContainerRemove(docker.Sdk.Ctx, containerId, container.RemoveOptions{})
			}
		}
		_, _ = dao.Site.Where(dao.Site.ID.Eq(siteRow.ID)).Updates(entity.Site{
			Status:  define.DockerImageBuildStatusError,
			Message: err.Error(),
		})
		_, _ = dao.Site.Where(dao.Site.ID.Eq(siteRow.ID)).Delete()
		self.JsonResponseWithError(http, err, 500)
		return
	}

	detail, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, containerId)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	_, _ = dao.Site.Where(dao.Site.ID.Eq(siteRow.ID)).Updates(&entity.Site{
		ContainerInfo: &accessor.SiteContainerInfoOption{
			Id:   containerId,
			Info: detail,
		},
		Status:  define.DockerImageBuildStatusSuccess,
		Message: "",
	})

	facade.GetEvent().Publish(event.ContainerCreateEvent, event.ContainerPayload{
		InspectInfo: &detail,
		Ctx:         http,
	})

	self.JsonResponseWithoutError(http, gin.H{
		"siteId":      siteRow.ID,
		"containerId": detail.ID,
	})
	return
}

func (self Site) GetList(http *gin.Context) {
	type ParamsValidate struct {
		SiteTitle string `json:"siteTitle" binding:"omitempty"`
		SiteName  string `json:"siteName"`
		Status    int32  `json:"status" binding:"omitempty,oneof=10 20 30"`
		IsDelete  bool   `json:"isDelete"`
	}

	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	query := dao.Site.Order(dao.Site.ID.Desc()).Where(gen.Cond(
		datatypes.JSONQuery("env").Equals(docker.Sdk.Name, "dockerEnvName"),
	)...)
	if params.Status != 0 {
		query = query.Where(dao.Site.Status.Eq(params.Status))
	}
	if params.SiteTitle != "" {
		query = query.Where(dao.Site.SiteTitle.Like("%" + params.SiteTitle + "%"))
	}
	if params.SiteName != "" {
		query = query.Where(dao.Site.SiteName.Like("%" + params.SiteName + "%"))
	}
	if params.IsDelete {
		if containerList, err := docker.Sdk.Client.ContainerList(docker.Sdk.Ctx, container.ListOptions{
			All: true,
		}); err == nil {
			names := make([]string, 0)
			for _, summary := range containerList {
				for _, name := range summary.Names {
					names = append(names, strings.TrimPrefix(name, "/"))
				}
			}
			query = query.Where(dao.Site.SiteName.NotIn(names...))
		}
		query = query.Unscoped().Where(dao.Site.DeletedAt.IsNotNull())
	}
	list, _ := query.Find()
	// 兼容非面板删除容器时，只取最后一条
	result := make([]*entity.Site, 0)
	for _, site := range list {
		if ok := function.InArrayWalk(result, func(item *entity.Site) bool {
			if item.SiteName == site.SiteName {
				return true
			} else {
				return false
			}
		}); !ok {
			result = append(result, site)
		}
	}
	self.JsonResponseWithoutError(http, gin.H{
		"list": result,
	})
	return
}

func (self Site) GetDetail(http *gin.Context) {
	type ParamsValidate struct {
		Id string `json:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	var siteRow *entity.Site
	if id, err := strconv.Atoi(params.Id); err == nil && len(params.Id) < 64 {
		siteRow, err = dao.Site.Unscoped().Where(dao.Site.ID.Eq(int32(id))).First()
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		// 更新容器的最新配置信息
		if containerInfo, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, params.Id); err == nil {
			siteRow.ContainerInfo = &accessor.SiteContainerInfoOption{
				Id:   containerInfo.ID,
				Info: containerInfo,
			}
			_ = dao.Site.Save(siteRow)
		}
	} else {
		containerInfo, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, params.Id)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		siteRow, _ = dao.Site.Where(gen.Cond(datatypes.JSONQuery("container_info").Equals(containerInfo.ID, "Id"))...).First()
		// 先使用容器名称查询，查询不到时通过 md5 再次查询
		//if siteRow == nil {
		//	siteRow, _ = dao.Site.Where(dao.Site.SiteName.Eq(strings.TrimLeft(containerInfo.Name, "/"))).First()
		//}

		siteName := strings.TrimPrefix(containerInfo.Name, "/")

		if siteRow == nil {
			// 站点不存在，表示容器没有通过面板创建，则创建出来站点数据
			// 保存配置信息，用于编辑和恢复
			siteRow = &entity.Site{
				SiteTitle: "",
				Env: &accessor.SiteEnvOption{
					Name:          siteName,
					DockerEnvName: docker.Sdk.Name,
				},
				SiteName: siteName,
			}
		}

		if siteRow.SiteName == "" {
			siteRow.SiteName = siteName
		}

		siteRow.ContainerInfo = &accessor.SiteContainerInfoOption{
			Id:   containerInfo.ID,
			Info: containerInfo,
		}

		// 为了兼容旧数据没有 containerInfo 字段的时候，将来可以删除掉
		if siteRow.ContainerInfo == nil || siteRow.ContainerInfo.Info.ContainerJSONBase == nil {
			siteRow.ContainerInfo = &accessor.SiteContainerInfoOption{
				Id:   params.Id,
				Info: containerInfo,
			}
		}

		for port, _ := range containerInfo.HostConfig.PortBindings {
			start, end, err := port.Range()
			fmt.Printf("GetDetail %v \n", start)
			fmt.Printf("GetDetail %v \n", end)
			fmt.Printf("GetDetail %v \n", err)
		}
		if siteRow.Env.DockerEnvName == "" && siteRow.Env != nil {
			siteRow.Env.DockerEnvName = docker.Sdk.Name
		}

		_ = dao.Site.Save(siteRow)
	}

	if siteRow.ContainerInfo != nil && siteRow.ContainerInfo.Info.Config.Image != "" {
		imageNameDetail := function.ImageTag(siteRow.ContainerInfo.Info.Config.Image)
		siteRow.ContainerInfo.Info.Config.Image = imageNameDetail.Uri()
	}

	self.JsonResponseWithoutError(http, siteRow)
	return
}

func (self Site) Delete(http *gin.Context) {
	type ParamsValidate struct {
		Id []int32 `json:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	_, err := dao.Site.Unscoped().Where(dao.Site.ID.In(params.Id...)).Delete()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Site) Restore(http *gin.Context) {
	type ParamsValidate struct {
		Name string `json:"name" binding:"required"`
	}

	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	siteRow, err := dao.Site.Unscoped().Where(dao.Site.SiteName.Eq(params.Name)).Last()
	if err != nil || siteRow.ContainerInfo == nil || siteRow.ContainerInfo.Info.Name == "" {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}

	if _, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, siteRow.ContainerInfo.Info.Name); err == nil {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonIdAlreadyExists, "name", siteRow.ContainerInfo.Info.Name), 500)
		return
	}

	out, err := docker.Sdk.Client.ContainerCreate(docker.Sdk.Ctx, siteRow.ContainerInfo.Info.Config, siteRow.ContainerInfo.Info.HostConfig, &network.NetworkingConfig{
		EndpointsConfig: siteRow.ContainerInfo.Info.NetworkSettings.Networks,
	}, &v1.Platform{}, siteRow.ContainerInfo.Info.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	_ = docker.Sdk.Client.ContainerStart(docker.Sdk.Ctx, siteRow.ContainerInfo.Info.Name, container.StartOptions{})
	info, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, out.ID)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	_, _ = dao.Site.Unscoped().Unscoped().Where(dao.Site.SiteName.Eq(params.Name)).Delete()

	err = dao.Site.Create(&entity.Site{
		SiteTitle: siteRow.SiteTitle,
		SiteName:  siteRow.SiteName,
		Env:       siteRow.Env,
		ContainerInfo: &accessor.SiteContainerInfoOption{
			Id:   out.ID,
			Info: info,
		},
		Status:    30,
		Message:   "",
		DeletedAt: gorm.DeletedAt{},
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"containerId": out.ID,
	})
}

func (self Site) Prune(http *gin.Context) {
	query := dao.Site.Order(dao.Site.ID.Desc()).Where(gen.Cond(
		datatypes.JSONQuery("env").Equals(docker.Sdk.Name, "dockerEnvName"),
	)...)
	if containerList, err := docker.Sdk.Client.ContainerList(docker.Sdk.Ctx, container.ListOptions{
		All: true,
	}); err == nil {
		names := make([]string, 0)
		for _, summary := range containerList {
			for _, name := range summary.Names {
				names = append(names, strings.TrimPrefix(name, "/"))
			}
		}
		query = query.Where(dao.Site.SiteName.NotIn(names...))
	}
	_, err := query.Unscoped().Where(
		dao.Site.DeletedAt.IsNotNull(),
		dao.Site.DeletedAt.Lte(gorm.DeletedAt{
			Time:  time.Now().AddDate(0, 0, -15),
			Valid: true,
		}),
	).Delete()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}
