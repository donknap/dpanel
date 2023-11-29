package controller

import (
	"encoding/json"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/src/core/err_handler"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
)

type Site struct {
	controller.Abstract
}

func (self Site) CreateByImage(http *gin.Context) {
	type ParamsValidate struct {
		SiteName   string `form:"siteName" binding:"required"`
		SiteId     string `form:"siteId" binding:"required"`
		SiteDomain string `form:"siteDomain" binding:"required"`
		Image      string `json:"image" binding:"required"`
		logic.ContainerRunParams
	}

	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	runParams := &logic.ContainerRunParams{
		Environment: params.Environment,
		Volumes:     params.Volumes,
		Ports:       params.Ports,
		Links:       params.Links,
	}

	err := dao.Q.Transaction(func(tx *dao.Query) error {
		site, _ := tx.Site.Where(dao.Site.SiteID.Eq(params.SiteId)).First()
		if site != nil {
			//return errors.New("站点已经存在，请更换标识")
		}
		param, err := json.Marshal(runParams)
		if err != nil {
			return err
		}
		containerRow := &entity.Container{
			Image:  params.Image,
			Params: string(param),
		}
		err = tx.Container.Create(containerRow)
		if err != nil {
			return err
		}
		siteRow := &entity.Site{
			ContainerID: containerRow.ID,
			SiteID:      params.SiteId,
			SiteName:    params.SiteName,
			SiteURL:     params.SiteDomain,
		}
		err = tx.Site.Create(siteRow)
		if err != nil {
			return err
		}
		return nil
	})

	if err_handler.Found(err) {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	task := logic.NewContainerTask()
	taskRow := &logic.CreateMessage{
		Name:      params.SiteId,
		Image:     params.Image,
		RunParams: runParams,
	}
	task.QueueCreate <- taskRow
	self.JsonSuccessResponse(http)
	return
}

func (self Site) GetList(http *gin.Context) {
	type ParamsValidate struct {
		Page     int    `form:"page,default=1" binding:"omitempty,gt=0"`
		SiteName string `form:"siteName" binding:"omitempty"`
		Sort     string `form:"sort,default=new" binding:"omitempty,oneof=hot new"`
	}

	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if params.Page < 1 {
		params.Page = 1
	}
	limit := 20

	query := dao.Site.Preload(dao.Site.Container.Select(dao.Container.ID, dao.Container.Image, dao.Container.Status))
	if params.SiteName != "" {
		query = query.Where(dao.Site.SiteName.Like("%" + params.SiteName + "%"))
	}
	list, total, _ := query.FindByPage((params.Page-1)*limit, limit)
	self.JsonResponseWithoutError(http, gin.H{
		"total": total,
		"page":  params.Page,
		"list":  list,
	})
	return
}
