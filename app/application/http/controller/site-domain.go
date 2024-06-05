package controller

import (
	"embed"
	"errors"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go-support/src/facade"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
	"html/template"
	"os"
	"strings"
)

var (
	defaultNetworkName = "dpanel-local"
	certFileName       = "%s.pem"
	keyFileName        = "%s-key.pem"
	vhostFileName      = "%s.conf"
)

type SiteDomain struct {
	controller.Abstract
}

func (self SiteDomain) Create(http *gin.Context) {
	type ParamsValidate struct {
		ContainerId               string `json:"containerId" binding:"required"`
		ServerName                string `json:"serverName" binding:"required"`
		Schema                    string `json:"schema" binding:"omitempty,oneof=http https"`
		Port                      int32  `json:"port" binding:"required"`
		EnableBlockCommonExploits bool   `json:"enableBlockCommonExploits"`
		EnableAssetCache          bool   `json:"enableAssetCache"`
		EnableWs                  bool   `json:"enableWs"`
		ExtraNginx                string `json:"extraNginx"`
		SslCrt                    string `json:"sslCrt"`
		SslKey                    string `json:"sslKey"`
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

	// 将当前容器加入到默认 dpanel-local 网络中，并指定 Hostname 用于 Nginx 反向代理
	_, err = docker.Sdk.Client.NetworkInspect(docker.Sdk.Ctx, defaultNetworkName, types.NetworkInspectOptions{})
	if err != nil {
		self.JsonResponseWithError(http, errors.New("DPanel 默认网络不存在，请重新安装或是新建&加入 "+defaultNetworkName+" 网络"), 500)
		return
	}
	hostname := fmt.Sprintf(docker.HostnameTemplate, strings.Trim(containerRow.Name, "/"))

	siteRow, _ := dao.Site.Where(dao.Site.ContainerInfo.Eq(&accessor.SiteContainerInfoOption{
		ID: params.ContainerId,
	})).First()

	if siteRow != nil {
		hostname = fmt.Sprintf(docker.HostnameTemplate, siteRow.SiteName)
	}

	docker.Sdk.Client.NetworkConnect(docker.Sdk.Ctx, defaultNetworkName, params.ContainerId, &network.EndpointSettings{
		Aliases: []string{
			hostname,
		},
	})

	var asset embed.FS
	err = facade.GetContainer().NamedResolve(&asset, "asset")
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	vhostFile, err := os.OpenFile(self.getNginxSettingPath()+fmt.Sprintf(vhostFileName, params.ServerName), os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0666)
	if err != nil {
		self.JsonResponseWithError(http, errors.New("nginx 配置目录不存在aqk"), 500)
		return
	}

	err = os.WriteFile(self.getNginxCertPath()+fmt.Sprintf(certFileName, params.ServerName), []byte(params.SslCrt), 0666)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	err = os.WriteFile(self.getNginxCertPath()+fmt.Sprintf(keyFileName, params.ServerName), []byte(params.SslKey), 0666)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer vhostFile.Close()

	type tplParams struct {
		ServerAddress             string
		Port                      int32
		ServerName                string
		EnableBlockCommonExploits bool
		EnableAssetCache          bool
		EnableWs                  bool
		ExtraNginx                template.HTML
		EnableSSL                 bool
	}
	parser, err := template.ParseFS(asset, "asset/nginx/*.tpl")
	err = parser.ExecuteTemplate(vhostFile, "vhost.tpl", tplParams{
		ServerAddress:             hostname,
		Port:                      params.Port,
		ServerName:                params.ServerName,
		EnableBlockCommonExploits: params.EnableBlockCommonExploits,
		EnableWs:                  params.EnableWs,
		EnableAssetCache:          params.EnableAssetCache,
		ExtraNginx:                template.HTML(params.ExtraNginx),
		EnableSSL:                 params.Schema == "https",
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	err = dao.SiteDomain.Create(&entity.SiteDomain{
		ServerName:  params.ServerName,
		Port:        params.Port,
		ContainerID: strings.Trim(containerRow.Name, "/"),
		Schema:      params.Schema,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	self.JsonSuccessResponse(http)
	return
}

func (self SiteDomain) GetList(http *gin.Context) {
	type ParamsValidate struct {
		ContainerId string `json:"containerId" binding:"required"`
		ServerName  string `json:"serverName"`
		Port        int32  `json:"port"`
		Page        int    `json:"page,default=1" binding:"omitempty,gt=0"`
		PageSize    int    `json:"pageSize" binding:"omitempty"`
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
	containerRow, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, params.ContainerId)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	query := dao.SiteDomain.Order(dao.SiteDomain.ID.Desc()).Where(dao.SiteDomain.ContainerID.Eq(strings.Trim(containerRow.Name, "/")))
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

func (self SiteDomain) GetDetail(http *gin.Context) {
	type ParamsValidate struct {
		Id int32 `json:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	domainRow, _ := dao.SiteDomain.Where(dao.SiteDomain.ID.Eq(params.Id)).First()
	if domainRow == nil {
		self.JsonResponseWithError(http, errors.New("域名不存在"), 500)
		return
	}

	vhost, err := os.ReadFile(self.getNginxSettingPath() + fmt.Sprintf(vhostFileName, domainRow.ServerName))
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	var sslCert []byte
	var sslKey []byte

	if domainRow.Schema == "https" {
		sslCert, err = os.ReadFile(self.getNginxCertPath() + fmt.Sprintf(certFileName, domainRow.ServerName))
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		sslKey, err = os.ReadFile(self.getNginxCertPath() + fmt.Sprintf(keyFileName, domainRow.ServerName))
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	self.JsonResponseWithoutError(http, gin.H{
		"domain":  domainRow,
		"vhost":   string(vhost),
		"sslCert": string(sslCert),
		"sslKey":  string(sslKey),
	})
	return
}

func (self SiteDomain) Delete(http *gin.Context) {
	type ParamsValidate struct {
		Id []int32 `json:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	list, _ := dao.SiteDomain.Where(dao.SiteDomain.ID.In(params.Id...)).Find()
	for _, item := range list {
		go os.Remove(self.getNginxSettingPath() + fmt.Sprintf(vhostFileName, item.ServerName))
		go os.Remove(self.getNginxCertPath() + fmt.Sprintf(certFileName, item.ServerName))
		go os.Remove(self.getNginxCertPath() + fmt.Sprintf(keyFileName, item.ServerName))
	}
	_, err := dao.SiteDomain.Where(dao.SiteDomain.ID.In(params.Id...)).Delete()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	// 如果没有域名，则退出默认网络
	if list != nil && len(list) > 0 {
		count, _ := dao.SiteDomain.Where(dao.SiteDomain.ContainerID.Eq(list[0].ContainerID)).Count()
		if count == 0 {
			err = docker.Sdk.Client.NetworkDisconnect(docker.Sdk.Ctx, defaultNetworkName, list[0].ContainerID, false)
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
		}
	}

	self.JsonSuccessResponse(http)
	return

}

func (self SiteDomain) getNginxSettingPath() string {
	return fmt.Sprintf("%s/nginx/proxy_host/", facade.GetConfig().Get("storage.local.path"))
}

func (self SiteDomain) getNginxCertPath() string {
	return fmt.Sprintf("%s/nginx/cert/", facade.GetConfig().Get("storage.local.path"))
}