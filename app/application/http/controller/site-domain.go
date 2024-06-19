package controller

import (
	"errors"
	"fmt"
	"github.com/docker/docker/api/types/network"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
	"html/template"
	"os"
	"strings"
)

var (
	defaultNetworkName = "dpanel-local"
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
	_, err = docker.Sdk.Client.NetworkInspect(docker.Sdk.Ctx, defaultNetworkName, network.InspectOptions{})
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

	if _, ok := containerRow.NetworkSettings.Networks[defaultNetworkName]; !ok {
		err = docker.Sdk.Client.NetworkConnect(docker.Sdk.Ctx, defaultNetworkName, params.ContainerId, &network.EndpointSettings{
			Aliases: []string{
				hostname,
			},
		})
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	domainSetting := &accessor.SiteDomainSettingOption{
		ServerName:                params.ServerName,
		Port:                      params.Port,
		ServerAddress:             hostname,
		EnableBlockCommonExploits: params.EnableBlockCommonExploits,
		EnableWs:                  params.EnableWs,
		EnableAssetCache:          params.EnableAssetCache,
		ExtraNginx:                template.HTML(params.ExtraNginx),
		EnableSSL:                 params.Schema == "https",
		TargetName:                function.GetMd5(params.ServerName),
		SslCrt:                    params.SslCrt,
		SslKey:                    params.SslKey,
	}

	err = logic.Site{}.MakeNginxConf(domainSetting)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	err = dao.SiteDomain.Create(&entity.SiteDomain{
		ServerName:  params.ServerName,
		ContainerID: strings.Trim(containerRow.Name, "/"),
		Setting:     domainSetting,
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

	vhost, err := os.ReadFile(logic.Site{}.GetNginxSettingPath() + fmt.Sprintf(logic.VhostFileName, domainRow.ServerName))
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	self.JsonResponseWithoutError(http, gin.H{
		"domain": domainRow,
		"vhost":  string(vhost),
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
		go os.Remove(logic.Site{}.GetNginxSettingPath() + fmt.Sprintf(logic.VhostFileName, item.ServerName))
		go os.Remove(logic.Site{}.GetNginxCertPath() + fmt.Sprintf(logic.CertFileName, item.ServerName))
		go os.Remove(logic.Site{}.GetNginxCertPath() + fmt.Sprintf(logic.KeyFileName, item.ServerName))
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

func (self SiteDomain) ApplyDomainCert(http *gin.Context) {
	type ParamsValidate struct {
		Id    int32  `json:"id" binding:"required"`
		Email string `json:"email" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	domain, _ := dao.SiteDomain.Where(dao.SiteDomain.ID.Eq(params.Id)).First()
	if domain == nil {
		self.JsonResponseWithError(http, errors.New("域名不存在"), 500)
		return
	}
	acmeUser, err := logic.NewAcmeUser(params.Email)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	client, err := lego.NewClient(lego.NewConfig(acmeUser))
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	err = client.Challenge.SetHTTP01Provider(logic.NewAcmeNginxProvider())
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	acmeUser.Registration = reg
	request := certificate.ObtainRequest{
		Domains: []string{domain.ServerName},
		Bundle:  true,
	}
	certificates, err := client.Certificate.Obtain(request)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	fmt.Printf("%v \n", string(certificates.Certificate))
}
