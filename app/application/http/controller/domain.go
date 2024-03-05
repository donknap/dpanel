package controller

import (
	"embed"
	"errors"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go-support/src/facade"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
	"html/template"
	"os"
)

var (
	confRootPath = "/Users/renchao/Workspace/data/dpanel/nginx/proxy_host"
)

type Domain struct {
	controller.Abstract
}

func (self Domain) Create(http *gin.Context) {
	type ParamsValidate struct {
		ContainerId               string `json:"containerId" binding:"required"`
		ServerName                string `json:"serverName" binding:"required"`
		Schema                    string `json:"schema" binding:"omitempty,oneof=http https"`
		Port                      int32  `json:"port" binding:"required"`
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
	siteDomainRow, _ := dao.SiteDomain.Where(dao.SiteDomain.ServerName.Eq(params.ServerName)).First()
	if siteDomainRow != nil {
		self.JsonResponseWithError(http, errors.New("域名已经存在"), 500)
		return
	}

	var asset embed.FS
	err = facade.GetContainer().NamedResolve(&asset, "asset")
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	vhostFile, err := os.OpenFile(confRootPath+"/"+params.ServerName+".conf", os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0666)
	defer vhostFile.Close()

	type tplParams struct {
		ServerAddress             string
		Port                      int32
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
		ServerName:                params.ServerName,
		EnableBlockCommonExploits: params.EnableBlockCommonExploits,
		EnableWs:                  params.EnableWs,
		EnableAssetCache:          params.EnableAssetCache,
		ExtraNginx:                params.ExtraNginx,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	err = dao.SiteDomain.Create(&entity.SiteDomain{
		ServerName:  params.ServerName,
		Port:        params.Port,
		ContainerID: containerRow.ID,
		Schema:      params.Schema,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	self.JsonSuccessResponse(http)
	return
}

func (self Domain) GetList(http *gin.Context) {
	type ParamsValidate struct {
		ServerName string `json:"serverName"`
		Port       int32  `json:"port"`
		Page       int    `json:"page,default=1" binding:"omitempty,gt=0"`
		PageSize   int    `json:"pageSize" binding:"omitempty"`
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

	query := dao.SiteDomain.Order(dao.SiteDomain.ID.Desc())
	if params.ServerName != "" {
		query = query.Where(dao.SiteDomain.ServerName.Like("%" + params.ServerName + "%"))
	}
	if params.Port > 0 {
		query = query.Where(dao.SiteDomain.Port.Eq(params.Port))
	}
	list, total, _ := query.FindByPage((params.Page-1)*params.PageSize, params.PageSize)

	self.JsonResponseWithoutError(http, gin.H{
		"total": total,
		"page":  params.Page,
		"list":  list,
	})
	return
}

func (self Domain) Delete(http *gin.Context) {
	type ParamsValidate struct {
		Id []int32 `json:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	list, _ := dao.SiteDomain.Where(dao.SiteDomain.ID.In(params.Id...)).Find()
	for _, item := range list {
		confFile := confRootPath + "/" + item.ServerName + ".conf"
		_ = os.Remove(confFile)
	}
	_, err := dao.SiteDomain.Where(dao.SiteDomain.ID.In(params.Id...)).Delete()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return

}
