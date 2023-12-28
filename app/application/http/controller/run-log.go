package controller

import (
	"errors"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
)

type RunLog struct {
	controller.Abstract
}

func (self RunLog) Run(http *gin.Context) {
	type ParamsValidate struct {
		Md5       string `form:"md5" binding:"required"`
		LineTotal int    `form:"lineTotal" binding:"required,number,oneof=50 100 200 500 1000"`
	}

	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	siteRow, _ := dao.Site.Where(dao.Site.ContainerInfo.Eq(&accessor.SiteContainerInfoOption{
		ID: params.Md5,
	})).First()
	if siteRow == nil {
		self.JsonResponseWithError(http, errors.New("站点不存在"), 500)
		return
	}
	if siteRow.ContainerInfo == nil {
		self.JsonResponseWithError(http, errors.New("当前站点并没有部署成功"), 500)
		return
	}

	builder := docker.Sdk.GetContainerLogBuilder()
	builder.WithContainerId(siteRow.ContainerInfo.Info.ID)
	builder.WithTail(params.LineTotal)
	content, err := builder.Execute()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"log": content,
	})
	return
}
