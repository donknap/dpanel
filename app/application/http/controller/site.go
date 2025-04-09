package controller

import (
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/types/event"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"gorm.io/datatypes"
	"gorm.io/gen"
	"gorm.io/gorm"
	"strconv"
	"strings"
)

type Site struct {
	controller.Abstract
}

func (self Site) CreateByImage(http *gin.Context) {
	type ParamsValidate struct {
		SiteTitle   string `json:"siteTitle"`
		SiteName    string `json:"siteName" binding:"required"`
		ImageName   string `json:"imageName" binding:"required"`
		ContainerId string `json:"containerId"`
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

	oldBindPort := make([]string, 0)
	oldContainerInfo, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, params.SiteName)
	if err == nil {
		for _, item := range oldContainerInfo.HostConfig.PortBindings {
			for _, value := range item {
				oldBindPort = append(oldBindPort, value.HostPort)
			}
		}
	}

	// 重新部署，先删掉之前的容器
	if params.ContainerId != "" {
		// 删除容器时，先把记录设置为软删除，部署失败后在回收站中可以查看
		_, _ = dao.Site.Where(dao.Site.SiteName.Eq(params.SiteName)).Delete()
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

	_, err = dao.Site.Unscoped().Where(dao.Site.SiteName.Eq(params.SiteName)).Delete()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	var siteRow *entity.Site
	siteRow, _ = dao.Site.Where(dao.Site.SiteName.Eq(params.SiteName)).First()
	if siteRow == nil {
		siteRow = &entity.Site{
			SiteName:      params.SiteName,
			SiteTitle:     params.SiteTitle,
			Env:           &buildParams,
			Status:        docker.ImageBuildStatusStop,
			ContainerInfo: &accessor.SiteContainerInfoOption{},
		}
		err := dao.Site.Create(siteRow)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	} else {
		_, _ = dao.Site.Select(dao.Site.ALL).Where(dao.Site.SiteName.Eq(params.SiteName)).Updates(&entity.Site{
			SiteTitle:     params.SiteTitle,
			Env:           &buildParams,
			Status:        docker.ImageBuildStatusStop,
			Message:       "",
			ContainerInfo: &accessor.SiteContainerInfoOption{},
			DeletedAt:     gorm.DeletedAt{},
		})
	}
	runTaskRow := &logic.CreateContainerOption{
		SiteName:    siteRow.SiteName,
		SiteId:      siteRow.ID,
		BuildParams: &buildParams,
	}
	containerId, err := logic.DockerTask{}.ContainerCreate(runTaskRow)
	if err != nil {
		if containerId != "" {
			// 如果容器在启动时发生错误，需要先删除掉
			_, err1 := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, containerId)
			if err1 == nil {
				_ = docker.Sdk.Client.ContainerRemove(docker.Sdk.Ctx, containerId, container.RemoveOptions{})
			}
		}
		_, _ = dao.Site.Where(dao.Site.ID.Eq(siteRow.ID)).Updates(entity.Site{
			Status:  docker.ImageBuildStatusError,
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
		Status:  docker.ImageBuildStatusSuccess,
		Message: "",
	})

	facade.GetEvent().Publish(event.ContainerCreateEvent, event.ContainerPayload{
		InspectInfo: &detail,
		Ctx:         http,
	})

	self.JsonResponseWithoutError(http, gin.H{"siteId": siteRow.ID})
	return
}

func (self Site) GetList(http *gin.Context) {
	type ParamsValidate struct {
		Page      int    `json:"page,default=1" binding:"omitempty,gt=0"`
		PageSize  int    `json:"pageSize" binding:"omitempty"`
		SiteTitle string `json:"siteTitle" binding:"omitempty"`
		SiteName  string `json:"siteName"`
		Status    int32  `json:"status" binding:"omitempty,oneof=10 20 30"`
		IsDelete  bool   `json:"isDelete"`
	}

	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if params.Page < 1 {
		params.Page = 1
	}

	query := dao.Site.Order(dao.Site.ID.Desc())
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
		query = query.Unscoped().Where(dao.Site.DeletedAt.IsNotNull())
	}
	if params.PageSize > 0 {
		list, total, _ := query.FindByPage((params.Page-1)*params.PageSize, params.PageSize)
		self.JsonResponseWithoutError(http, gin.H{
			"total": total,
			"page":  params.Page,
			"list":  list,
		})
	} else {
		list, _ := query.Find()
		self.JsonResponseWithoutError(http, gin.H{
			"total": len(list),
			"page":  params.Page,
			"list":  list,
		})
	}
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

	var containerName string
	var siteRow *entity.Site
	var runOption accessor.SiteEnvOption

	if id, err := strconv.Atoi(params.Id); err == nil && len(params.Id) < 64 {
		siteRow, err = dao.Site.Unscoped().Where(dao.Site.ID.Eq(int32(id))).First()
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		containerName = siteRow.SiteName
	} else {
		containerInfo, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, params.Id)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		containerName = params.Id
		// 先使用容器名称查询，查询不到时通过 md5 再次查询
		siteRow, _ = dao.Site.Where(dao.Site.SiteName.Eq(strings.TrimLeft(containerInfo.Name, "/"))).First()
		if siteRow == nil {
			siteRow, _ = dao.Site.Where(gen.Cond(datatypes.JSONQuery("container_info").Equals(params.Id, "Id"))...).First()
		}
		runOption, err = logic.Site{}.GetEnvOptionByContainer(containerName)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		// 站点不存在，表示容器没有通过面板创建，则创建出来站点数据
		if siteRow == nil {
			siteRow = &entity.Site{
				ContainerInfo: &accessor.SiteContainerInfoOption{
					Id:   params.Id,
					Info: containerInfo,
				},
				SiteTitle: "",
				SiteName:  strings.TrimLeft(containerInfo.Name, "/"),
			}
			runOption.Command = ""
			runOption.Entrypoint = ""
			runOption.WorkDir = ""
			siteRow.Env = &runOption
			_ = dao.Site.Save(siteRow)
		} else if siteRow.ContainerInfo == nil || siteRow.ContainerInfo.Info.ContainerJSONBase == nil {
			siteRow.ContainerInfo = &accessor.SiteContainerInfoOption{
				Id:   params.Id,
				Info: containerInfo,
			}
			_ = dao.Site.Save(siteRow)
		}
	}

	// 站点有些数据需要从容器信息上获取，这里是否可以直接使用 containerInfo ?
	if !function.IsEmptyArray(runOption.Network) {
		siteRow.Env.Network = runOption.Network
	}
	if !function.IsEmptyArray(runOption.CapAdd) {
		siteRow.Env.CapAdd = runOption.CapAdd
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
