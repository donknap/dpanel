package controller

import (
	"embed"
	"errors"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go-support/src/facade"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
	"gorm.io/gorm"
	"html/template"
	"log/slog"
	"net"
	"os"
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
		docker.Sdk.Client.ContainerStop(docker.Sdk.Ctx, params.SiteName, container.StopOptions{})
		err := docker.Sdk.Client.ContainerRemove(docker.Sdk.Ctx, params.SiteName, container.RemoveOptions{})
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			slog.Debug("remove container", "name", params.SiteName, "error", err.Error())
			return
		}
	}
	if params.Ports != nil {
		var checkPorts []string
		for _, port := range params.Ports {
			if port.Type == "port" {
				// 检测端口是否可以正常绑定
				listener, err := net.Listen("tcp", "0.0.0.0:"+port.Host)
				if err != nil {
					self.JsonResponseWithError(http, err, 500)
					return
				}
				listener.Close()
				checkPorts = append(checkPorts, port.Host)
			} else {
				self.JsonResponseWithError(http, errors.New("不支持的暴露类型"), 500)
				return
			}
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
		runOption, err := logic.Site{}.GetEnvOptionByContainer(params.Md5)
		siteRow.Env = &runOption
	}
	self.JsonResponseWithoutError(http, siteRow)
	return
}

func (self Site) Delete(http *gin.Context) {
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
	if !function.IsEmptyArray(containerInfo.HostConfig.Links) {
		params.DeleteLink = true
	} else {
		params.DeleteLink = false
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
		_, err = docker.Sdk.Client.ImageRemove(docker.Sdk.Ctx, containerInfo.Image, types.ImageRemoveOptions{})
	}
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	siteRow, _ := dao.Site.Where(dao.Site.ContainerInfo.Eq(&accessor.SiteContainerInfoOption{
		ID: params.Md5,
	})).First()

	if siteRow != nil {
		docker.Sdk.Client.NetworkRemove(docker.Sdk.Ctx, siteRow.SiteName)
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

func (self Site) CreateDomain(http *gin.Context) {
	type ParamsValidate struct {
		ContainerId               string `json:"containerId" binding:"required"`
		Domain                    string `json:"domain" binding:"required"`
		Schema                    string `json:"schema" binding:"omitempty,oneof=http https"`
		Port                      int    `json:"port" binding:"required"`
		EnableBlockCommonExploits bool   `json:"enableBlockCommonExploits"`
		EnableAssetCache          bool   `json:"enableAssetCache"`
		EnableWs                  bool   `json:"enableWs"`
		ExtraNginx                string `json:"extraNginx"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	containerRow, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, params.ContainerId)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	var asset embed.FS
	err = facade.GetContainer().NamedResolve(&asset, "asset")
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	confRootPath := "/Users/renchao/Workspace/data/dpanel/nginx/proxy_host"
	vhostFile, err := os.OpenFile(confRootPath+"/"+params.Domain+".conf", os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0666)
	defer vhostFile.Close()

	type tplParams struct {
		ServerAddress             string
		Port                      int
		ServerName                string
		EnableBlockCommonExploits bool
		EnableAssetCache          bool
		EnableWs                  bool
		ExtraNginx                string
	}
	parser, err := template.ParseFS(asset, "asset/nginx/*.tpl")
	err = parser.ExecuteTemplate(vhostFile, "vhost.tpl", tplParams{
		ServerAddress:             containerRow.NetworkSettings.DefaultNetworkSettings.IPAddress,
		Port:                      params.Port,
		ServerName:                params.Domain,
		EnableBlockCommonExploits: params.EnableBlockCommonExploits,
		EnableWs:                  params.EnableWs,
		EnableAssetCache:          params.EnableAssetCache,
		ExtraNginx:                params.ExtraNginx,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

}
