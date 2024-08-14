package controller

import (
	"errors"
	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"gorm.io/gorm"
	"log/slog"
	"net"
)

type Site struct {
	controller.Abstract
}

func (self Site) CreateByImage(http *gin.Context) {
	type ParamsValidate struct {
		SiteTitle string `json:"siteTitle" binding:"required"`
		SiteName  string `json:"siteName" binding:"required"`
		ImageName string `json:"imageName" binding:"required"`
		Id        int    `json:"id"`
		Md5       string `json:"md5"`
		accessor.SiteEnvOption
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	// 重新部署，先删掉之前的容器
	if params.Id != 0 || params.Md5 != "" {
		notice.Message{}.Info("containerCreate", "正在停止旧容器")
		_, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, params.SiteName)
		if err == nil {
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
		}
	}

	if params.Ports != nil {
		var checkPorts []string
		for _, port := range params.Ports {
			// 检测端口是否可以正常绑定
			listener, err := net.Listen("tcp", "0.0.0.0:"+port.Host)
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
			listener.Close()
			checkPorts = append(checkPorts, port.Host)
		}
		if checkPorts != nil {
			item, _ := docker.Sdk.ContainerByField("publish", checkPorts...)
			if len(item) > 0 {
				self.JsonResponseWithError(http, errors.New("绑定的外部端口已经被其它容器占用，请更换其它端口"), 500)
				return
			}
		}
	}
	imageInfo, _, err := docker.Sdk.Client.ImageInspectWithRaw(docker.Sdk.Ctx, params.ImageName)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	runParams := accessor.SiteEnvOption{
		Environment:    params.Environment,
		Links:          params.Links,
		Ports:          params.Ports,
		Volumes:        params.Volumes,
		VolumesDefault: params.VolumesDefault,
		ImageName:      params.ImageName,
		ImageId:        imageInfo.ID,
		Privileged:     params.Privileged,
		Restart:        params.Restart,
		Cpus:           params.Cpus,
		Memory:         params.Memory,
		ShmSize:        params.ShmSize,
		WorkDir:        params.WorkDir,
		User:           params.User,
		Command:        params.Command,
		Entrypoint:     params.Entrypoint,
		Network:        params.Network,
		UseHostNetwork: params.UseHostNetwork,
		BindIpV6:       params.BindIpV6,
	}
	dao.Site.Unscoped().Where(dao.Site.SiteName.Eq(params.SiteName)).Delete()

	var siteRow *entity.Site
	siteRow, _ = dao.Site.Where(dao.Site.SiteName.Eq(params.SiteName)).First()
	if siteRow == nil {
		siteRow = &entity.Site{
			SiteName:  params.SiteName,
			SiteTitle: params.SiteTitle,
			Env:       &runParams,
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
			Env:       &runParams,
			Status:    logic.StatusStop,
			Message:   "",
			DeletedAt: gorm.DeletedAt{},
		})
	}
	runTaskRow := &logic.CreateMessage{
		SiteName:  siteRow.SiteName,
		SiteId:    siteRow.ID,
		RunParams: &runParams,
	}
	err = logic.DockerTask{}.ContainerCreate(runTaskRow)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
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
