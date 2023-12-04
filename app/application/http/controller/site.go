package controller

import (
	"errors"
	"fmt"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/src/core/err_handler"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
	"strings"
)

type Site struct {
	controller.Abstract
}

func (self Site) CreateByImage(http *gin.Context) {
	type ParamsValidate struct {
		SiteName string `form:"siteName" binding:"required"`
		SiteUrl  string `form:"siteUrl" binding:"required,url"`
		Image    string `json:"image" binding:"required"`
		Type     string `json:"type" binding:"required,oneof=system site"`
		accessor.SiteEnvOption
	}

	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	runParams := accessor.SiteEnvOption{
		Environment: params.Environment,
		Volumes:     params.Volumes,
		Ports:       params.Ports,
		Links:       params.Links,
	}
	// 如果是系统组件，域名相关配置可以去掉
	if params.Type == logic.SITE_TYPE_SYSTEM {
		params.SiteUrl = ""
	}
	siteUrlExt := &accessor.SiteUrlExtOption{}
	siteUrlExt.Url = append(siteUrlExt.Url, params.SiteUrl)

	siteRow := &entity.Site{
		SiteID:     "",
		SiteName:   params.SiteName,
		SiteURL:    params.SiteUrl,
		SiteURLExt: siteUrlExt,
		Env:        &runParams,
		Status:     logic.STATUS_STOP,
		Type:       logic.SiteTypeValue[params.Type],
	}

	err := dao.Q.Transaction(
		func(tx *dao.Query) error {
			if params.Type == logic.SITE_TYPE_SITE {
				site, _ := tx.Site.Where(dao.Site.SiteURL.Eq(params.SiteUrl)).First()
				if site != nil {
					return errors.New("站点域名已经绑定其它站，请更换域名")
				}
			}

			if params.Image != "" {
				imageArr := strings.Split(
					params.Image+":",
					":",
				)
				containerRow := &entity.Container{
					Image:         imageArr[0],
					Version:       imageArr[1],
					Dockerfile:    "",
					ContainerInfo: &accessor.ContainerInfoOption{},
				}
				err := tx.Container.Create(containerRow)
				if err != nil {
					return err
				}
				siteRow.ContainerID = containerRow.ID

			}
			err := tx.Site.Create(siteRow)
			if err != nil {
				return err
			}
			return nil
		},
	)

	siteRow.SiteID = fmt.Sprintf("dpanel-%s-%d-%s", params.Type, siteRow.ID, function.GetRandomString(10))
	dao.Site.Where(dao.Site.ID.Eq(siteRow.ID)).Updates(siteRow)
	if err_handler.Found(err) {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	task := logic.NewContainerTask()
	runTaskRow := &logic.CreateMessage{
		Name:      siteRow.SiteID,
		SiteId:    siteRow.ID,
		Image:     params.Image,
		RunParams: &runParams,
	}
	task.QueueCreate <- runTaskRow
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{"siteId": siteRow.ID})
	return
}

func (self Site) GetList(http *gin.Context) {
	type ParamsValidate struct {
		Page     int    `form:"page,default=1" binding:"omitempty,gt=0"`
		PageSize int    `form:"pageSize" binding:"omitempty"`
		SiteName string `form:"siteName" binding:"omitempty"`
		Sort     string `form:"sort,default=new" binding:"omitempty,oneof=hot new"`
		Type     string `form:"type" binding:"oneof=system site"`
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
	query = query.Preload(
		dao.Site.Container.Select(
			dao.Container.ID,
			dao.Container.Image,
			dao.Container.Status,
			dao.Container.Version,
			dao.Container.ContainerInfo,
		),
	)
	if params.Type != "" {
		query = query.Where(dao.Site.Type.Eq(logic.SiteTypeValue[params.Type]))
	}
	if params.SiteName != "" {
		query = query.Where(dao.Site.SiteName.Like("%" + params.SiteName + "%"))
	}
	list, total, _ := query.FindByPage((params.Page-1)*params.PageSize, params.PageSize)

	if list != nil {
		sdk, err := docker.NewDockerClient()
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}

		var containerNameList []string
		for _, site := range list {
			containerNameList = append(containerNameList, site.SiteID)
		}
		containerInfoList, err := sdk.ContainerByField("name", containerNameList...)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		for _, site := range list {
			if item, ok := containerInfoList[site.SiteID]; ok {
				site.Container.ContainerInfo = (*accessor.ContainerInfoOption)(item)
			}
		}
	}
	self.JsonResponseWithoutError(http, gin.H{
		"total": total,
		"page":  params.Page,
		"list":  list,
	})
	return
}

func (self Site) GetDetail(http *gin.Context) {
	type ParamsValidate struct {
		Id int32 `form:"siteId" binding:"required"`
	}

	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	siteRow, _ := dao.Site.Where(dao.Site.ID.Eq(params.Id)).Preload(dao.Site.Container).First()
	if siteRow == nil {
		self.JsonResponseWithError(http, errors.New("站点不存在"), 500)
		return
	}
	// 更新容器信息
	self.JsonResponseWithoutError(http, siteRow)
	return

}
