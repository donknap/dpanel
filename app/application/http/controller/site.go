package controller

import (
	"errors"
	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"gorm.io/gorm"
	"log/slog"
	"net"
	"strings"
)

type Site struct {
	controller.Abstract
}

func (self Site) CreateByImage(http *gin.Context) {
	type ParamsValidate struct {
		Id          int32  `json:"id"`
		SiteTitle   string `json:"siteTitle" binding:"required"`
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

	for _, itemDefault := range buildParams.VolumesDefault {
		for _, item := range buildParams.Volumes {
			if item.Dest == itemDefault.Dest {
				self.JsonResponseWithError(http, errors.New("容器内的 "+item.Dest+" 目录重复绑定存储"), 500)
				return
			}
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

	if buildParams.Ports != nil {
		var checkPorts []string
		for _, port := range buildParams.Ports {
			// 如果容器绑定过该端口，则不需要再次检测
			if function.InArray(oldBindPort, port.Host) {
				continue
			}
			listener, err := net.Listen("tcp", "0.0.0.0:"+port.Host)
			if err != nil {
				self.JsonResponseWithError(http, errors.New(port.Host+"绑定的外部端口已经被其它容器占用，请更换。"), 500)
				return
			}
			listener.Close()
			checkPorts = append(checkPorts, port.Host)
		}
		// 没有绑定宿主机的端口，有可能被未启动的容器绑定，这里再次检查一下
		if checkPorts != nil {
			hasPortContainer, _ := docker.Sdk.ContainerByField("publish", checkPorts...)
			if len(hasPortContainer) > 0 {
				names := make([]string, 0)
				for _, item := range hasPortContainer {
					names = append(names, item.Names[0])
				}
				self.JsonResponseWithError(http, errors.New("绑定的外部端口已经被 "+strings.Join(names, "/")+" 容器占用，请更换其它端口"), 500)
				return
			}
		}
	}

	// 重新部署，先删掉之前的容器
	if params.Id != 0 || params.ContainerId != "" {
		_ = notice.Message{}.Info("containerCreate", "正在停止旧容器")
		if oldContainerInfo.ID != "" {
			err := docker.Sdk.Client.ContainerStop(docker.Sdk.Ctx, params.SiteName, container.StopOptions{})
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
			err = docker.Sdk.Client.ContainerRemove(docker.Sdk.Ctx, params.SiteName, container.RemoveOptions{})
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				slog.Debug("remove container", "name", params.SiteName, "error", err.Error())
				return
			}
			// 删除容器时，先把记录设置为软删除，部署失败后在回收站中可以查看
			_, _ = dao.Site.Where(dao.Site.SiteName.Eq(params.SiteName)).Delete()
		}
	}

	imageInfo, _, err := docker.Sdk.Client.ImageInspectWithRaw(docker.Sdk.Ctx, params.ImageName)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	buildParams.ImageId = imageInfo.ID
	dao.Site.Unscoped().Where(dao.Site.SiteName.Eq(params.SiteName)).Delete()

	var siteRow *entity.Site
	siteRow, _ = dao.Site.Where(dao.Site.SiteName.Eq(params.SiteName)).First()
	if siteRow == nil {
		siteRow = &entity.Site{
			SiteName:  params.SiteName,
			SiteTitle: params.SiteTitle,
			Env:       &buildParams,
			Status:    logic.StatusStop,
			ContainerInfo: &accessor.SiteContainerInfoOption{
				ID: "",
			},
		}
		err := dao.Site.Create(siteRow)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	} else {
		dao.Site.Select(dao.Site.ALL).Where(dao.Site.SiteName.Eq(params.SiteName)).Updates(&entity.Site{
			SiteTitle: params.SiteTitle,
			Env:       &buildParams,
			Status:    logic.StatusStop,
			Message:   "",
			DeletedAt: gorm.DeletedAt{},
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
			Status:  accessor.StatusError,
			Message: err.Error(),
		})
		_, _ = dao.Site.Where(dao.Site.ID.Eq(siteRow.ID)).Delete()
		self.JsonResponseWithError(http, err, 500)
		return
	}

	dao.Site.Where(dao.Site.ID.Eq(siteRow.ID)).Updates(&entity.Site{
		ContainerInfo: &accessor.SiteContainerInfoOption{
			ID: containerId,
		},
		Status:  accessor.StatusSuccess,
		Message: "",
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
	if params.PageSize < 1 {
		params.PageSize = 10
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
	list, total, _ := query.FindByPage((params.Page-1)*params.PageSize, params.PageSize)

	self.JsonResponseWithoutError(http, gin.H{
		"total": total,
		"page":  params.Page,
		"list":  list,
	})
	return
}

func (self Site) GetDetail(http *gin.Context) {
	type ParamsValidate struct {
		Md5 string `form:"md5" binding:"required_if=Id 0"`
		Id  int32  `json:"id"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	var siteRow *entity.Site
	if params.Id != 0 {
		siteRow, _ = dao.Site.Unscoped().Where(dao.Site.ID.Eq(params.Id)).First()
	} else {
		siteRow, _ = dao.Site.Where(dao.Site.ContainerInfo.Eq(&accessor.SiteContainerInfoOption{
			ID: params.Md5,
		})).First()
	}
	if params.Md5 == "" {
		self.JsonResponseWithoutError(http, siteRow)
		return
	}
	runOption, err := logic.Site{}.GetEnvOptionByContainer(params.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	// 站点不存在，返回容器那部分，并建立 env 字段的内容
	if siteRow == nil {
		info, err := docker.Sdk.ContainerInfo(params.Md5)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		siteRow = &entity.Site{
			ContainerInfo: &accessor.SiteContainerInfoOption{
				ID:   params.Md5,
				Info: &info,
			},
			SiteTitle: info.Name,
			SiteName:  info.Name,
		}

		runOption.Command = ""
		runOption.Entrypoint = ""
		runOption.WorkDir = ""
		siteRow.Env = &runOption
	}
	siteRow.Env.Network = runOption.Network
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

func (self Site) UpdateTitle(http *gin.Context) {
	type ParamsValidate struct {
		Md5   string `json:"md5" binding:"required"`
		Title string `json:"title" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	siteRow, _ := dao.Site.Where(dao.Site.ContainerInfo.Eq(&accessor.SiteContainerInfoOption{
		ID: params.Md5,
	})).First()
	if siteRow != nil {
		dao.Site.Where(dao.Site.ContainerInfo.Eq(&accessor.SiteContainerInfoOption{
			ID: params.Md5,
		})).Updates(&entity.Site{
			SiteTitle: params.Title,
		})
	} else {
		dao.Site.Create(&entity.Site{
			SiteTitle: params.Title,
			ContainerInfo: &accessor.SiteContainerInfoOption{
				ID: params.Md5,
			},
		})
	}
	self.JsonSuccessResponse(http)
	return
}
