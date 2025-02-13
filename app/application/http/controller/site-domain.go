package controller

import (
	"errors"
	"fmt"
	"github.com/docker/docker/api/types/network"
	"github.com/donknap/dpanel/app/application/logic"
	logic2 "github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/acme"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/exec"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"html/template"
	"io"
	"log/slog"
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
		Hostname                  string `json:"hostname"`
		Port                      int32  `json:"port" binding:"required"`
		EnableBlockCommonExploits bool   `json:"enableBlockCommonExploits"`
		EnableAssetCache          bool   `json:"enableAssetCache"`
		EnableWs                  bool   `json:"enableWs"`
		ExtraNginx                string `json:"extraNginx"`
		SslCrt                    string `json:"sslCrt"`
		SslKey                    string `json:"sslKey"`
		SslCrtRenewTime           string `json:"sslCrtRenewTime"`
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

	var siteDomainRow *entity.SiteDomain
	var domainSetting *accessor.SiteDomainSettingOption

	siteDomainRow, _ = dao.SiteDomain.Where(dao.SiteDomain.ServerName.Eq(params.ServerName)).First()
	if siteDomainRow != nil {
		self.JsonResponseWithError(http, errors.New("域名已经存在"), 500)
		return
	}
	// 将当前容器加入到默认 dpanel-local 网络中，并指定 Hostname 用于 Nginx 反向代理
	dpanelContainerInfo, err := docker.Sdk.ContainerInfo(facade.GetConfig().GetString("app.name"))
	if err != nil {
		self.JsonResponseWithError(http, errors.New("当前环境没有 dpanel 面板，或是创建的面板容器名称非默认的 dpanel，请重建并通过环境变量 APP_NAME 指定新的名称。"), 500)
		return
	}

	var hostname string
	if params.Hostname != "" {
		hostname = params.Hostname
	} else {
		if _, ok := dpanelContainerInfo.NetworkSettings.Networks[defaultNetworkName]; !ok {
			_, err = docker.Sdk.Client.NetworkInspect(docker.Sdk.Ctx, defaultNetworkName, network.InspectOptions{})
			if err != nil {
				_, err = docker.Sdk.Client.NetworkCreate(docker.Sdk.Ctx, defaultNetworkName, network.CreateOptions{
					Driver: "bridge",
					Options: map[string]string{
						"name": defaultNetworkName,
					},
					EnableIPv6: function.PtrBool(false),
				})
				if err != nil {
					self.JsonResponseWithError(http, errors.New("创建 DPanel 默认网络失败，请重新安装并新建&加入 "+defaultNetworkName+" 网络"), 500)
					return
				}
			}
			// 假如是自身绑定域名，不加入网络，在下面统一处理
			if dpanelContainerInfo.ID != containerRow.ID {
				err = docker.Sdk.Client.NetworkConnect(docker.Sdk.Ctx, defaultNetworkName, dpanelContainerInfo.ID, &network.EndpointSettings{})
				if err != nil {
					self.JsonResponseWithError(http, errors.New("创建 DPanel 默认网络失败，请重新安装并新建&加入 "+defaultNetworkName+" 网络"), 500)
					return
				}
			}
		}
		hostname = fmt.Sprintf(docker.HostnameTemplate, strings.Trim(containerRow.Name, "/"))
		siteRow, _ := dao.Site.Where(dao.Site.ContainerInfo.Eq(&accessor.SiteContainerInfoOption{
			ID: params.ContainerId,
		})).First()

		if siteRow != nil && siteRow.SiteName != "" {
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
	}

	domainSetting = &accessor.SiteDomainSettingOption{
		ServerName:                params.ServerName,
		ServerAddress:             hostname,
		Port:                      params.Port,
		EnableBlockCommonExploits: params.EnableBlockCommonExploits,
		EnableWs:                  params.EnableWs,
		EnableAssetCache:          params.EnableAssetCache,
		EnableSSL:                 false,
		ExtraNginx:                template.HTML(params.ExtraNginx),
		TargetName:                function.GetMd5(params.ServerName),
	}
	if params.SslKey != "" && params.SslCrt != "" && params.SslCrtRenewTime != "" {
		domainSetting.SslKey = params.SslKey
		domainSetting.SslCrt = params.SslCrt
		domainSetting.SslCrtRenewTime = params.SslCrtRenewTime
		domainSetting.EnableSSL = true
	}

	err = logic.Site{}.MakeNginxConf(domainSetting)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	err = dao.SiteDomain.Create(&entity.SiteDomain{
		ServerName:  params.ServerName,
		ContainerID: containerRow.Name,
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
		ContainerId string `json:"containerId"`
		ServerName  string `json:"serverName"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	query := dao.SiteDomain.Order(dao.SiteDomain.ID.Desc())
	if params.ContainerId != "" {
		containerRow, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, params.ContainerId)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		query = query.Where(dao.SiteDomain.ContainerID.Eq(containerRow.Name))
	}
	if params.ServerName != "" {
		query = query.Where(dao.SiteDomain.ServerName.Like("%" + params.ServerName + "%"))
	}
	list, _ := query.Find()

	for key, domain := range list {
		if domain.Setting != nil && domain.Setting.EnableSSL && domain.Setting.SslCrtKey != "" {
			certInfo := logic.Acme{}.Info(domain.Setting.SslCrtKey)
			list[key].Setting.SslCrtRenewTime = certInfo.RenewTimeStr
		}
	}

	self.JsonResponseWithoutError(http, gin.H{
		"list": list,
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
	vhost, err := logic.Site{}.GetSiteNginxSetting(domainRow.ServerName).GetConfContent()
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
		go logic.Site{}.GetSiteNginxSetting(item.ServerName).RemoveAll()
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
			// 如果只有dpanel-local一个网络则保留
			containerInfo, err := docker.Sdk.ContainerInfo(list[0].ContainerID)
			if err == nil && len(containerInfo.NetworkSettings.Networks) > 1 {
				err = docker.Sdk.Client.NetworkDisconnect(docker.Sdk.Ctx, defaultNetworkName, list[0].ContainerID, false)
				if err != nil {
					self.JsonResponseWithError(http, err, 500)
					return
				}
			}
		}
	}

	self.JsonSuccessResponse(http)
	return

}

func (self SiteDomain) ApplyDomainCert(http *gin.Context) {
	type ParamsValidate struct {
		Domain      []string `json:"domain" binding:"required"`
		Email       string   `json:"email" binding:"required"`
		CertServer  string   `json:"certServer" binding:"required" oneof:"zerossl letsencrypt"`
		AuthUpgrade bool     `json:"authUpgrade"`
		Renew       bool     `json:"renew"`
		Debug       bool     `json:"debug"`
		DnsApi      string   `json:"dnsApi"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	options := []acme.Option{
		acme.WithDomainList(params.Domain...),
		acme.WithEmail(params.Email),
		acme.WithCertServer(params.CertServer),
	}

	if params.AuthUpgrade {
		options = append(options, acme.WithAutoUpgrade())
	}

	if params.Renew {
		options = append(options, acme.WithRenew(), acme.WithForce())
	} else {
		options = append(options, acme.WithIssue())
	}

	if params.Debug {
		options = append(options, acme.WithDebug())
	}

	if params.DnsApi != "" {
		if params.DnsApi == "nginx" {
			options = append(options, acme.WithDnsNginx())
		} else {
			dnsApiList := make([]accessor.DnsApi, 0)
			logic2.Setting{}.GetByKey(logic2.SettingGroupSetting, logic2.SettingGroupSettingDnsApi, &dnsApiList)
			if exists, i := function.IndexArrayWalk(dnsApiList, func(i accessor.DnsApi) bool {
				return i.ServerName == params.DnsApi
			}); exists {
				options = append(options, acme.WithDnsApi(dnsApiList[i]))
			}
		}
	}

	if facade.GetConfig().GetString("app.env") == "debug" {
		_ = os.Setenv(acme.EnvOverrideCommandName, "/Users/renchao/.acme.sh/acme.sh")
	} else {
		options = append(options, acme.WithConfigHomePath("/dpanel/acme"))
	}

	builder, err := acme.New(options...)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	response, err := builder.Run()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	wsBuffer := ws.NewProgressPip(ws.MessageTypeDomainApply)
	defer wsBuffer.Close()
	_, err = io.Copy(wsBuffer, response)
	if err != nil {
		slog.Error("compose", "deploy copy error", err)
	}
	self.JsonSuccessResponse(http)
	return
}

func (self SiteDomain) UpdateDomain(http *gin.Context) {
	type ParamsValidate struct {
		Id                        int32  `json:"id" binding:"required"`
		SslCrt                    string `json:"sslCrt"`
		SslKey                    string `json:"sslKey"`
		SslCrtRenewTime           string `json:"sslCrtRenewTime"`
		Port                      int32  `json:"port"`
		EnableBlockCommonExploits bool   `json:"enableBlockCommonExploits"`
		EnableAssetCache          bool   `json:"enableAssetCache"`
		EnableWs                  bool   `json:"enableWs"`
		ExtraNginx                string `json:"extraNginx"`
		Hostname                  string `json:"hostname"`
	}

	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	siteDomainRow, _ := dao.SiteDomain.Where(dao.SiteDomain.ID.Eq(params.Id)).First()
	if siteDomainRow == nil {
		self.JsonResponseWithError(http, errors.New("域名不存在，请先添加域名"), 500)
		return
	}
	// 站点基本信息
	if params.Port != 0 {
		siteDomainRow.Setting.Port = params.Port
	}

	siteDomainRow.Setting.EnableBlockCommonExploits = params.EnableBlockCommonExploits
	siteDomainRow.Setting.EnableAssetCache = params.EnableAssetCache
	siteDomainRow.Setting.EnableWs = params.EnableWs
	siteDomainRow.Setting.ServerAddress = params.Hostname

	if params.ExtraNginx != "" {
		siteDomainRow.Setting.ExtraNginx = template.HTML(params.ExtraNginx)
	}
	// 手动导入证书
	if params.SslCrt != "" && params.SslKey != "" && siteDomainRow.Setting.AutoSsl == false {
		siteDomainRow.Setting.SslCrt = params.SslCrt
		siteDomainRow.Setting.SslKey = params.SslKey
		siteDomainRow.Setting.AutoSsl = false
		siteDomainRow.Setting.SslCrtRenewTime = params.SslCrtRenewTime
		siteDomainRow.Setting.EnableSSL = true
	}
	err := logic.Site{}.MakeNginxConf(siteDomainRow.Setting)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	_, err = dao.SiteDomain.Where(dao.SiteDomain.ID.Eq(params.Id)).Updates(&entity.SiteDomain{
		Setting: siteDomainRow.Setting,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}

func (self SiteDomain) RestartNginx(http *gin.Context) {
	cmd, _ := exec.New(
		exec.WithCommandName("nginx"),
		exec.WithArgs("-s", "stop"),
	)
	out := cmd.RunWithResult()
	slog.Debug("site domain stop nginx", "out", out)

	cmd, _ = exec.New(
		exec.WithCommandName("nginx"),
	)
	out = cmd.RunWithResult()
	slog.Debug("site domain stop nginx", "out", out)

	self.JsonSuccessResponse(http)
	return
}

func (self SiteDomain) DnsApi(http *gin.Context) {
	type ParamsValidate struct {
		Account []accessor.DnsApi `json:"account"`
		User    []accessor.DnsApi `json:"user"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	dnsApi := make([]accessor.DnsApi, 0)
	logic2.Setting{}.GetByKey(logic2.SettingGroupSetting, logic2.SettingGroupSettingDnsApi, &dnsApi)

	if !function.IsEmptyArray(params.Account) || !function.IsEmptyArray(params.User) {
		dnsApi = make([]accessor.DnsApi, 0)
		for _, item := range params.Account {
			if exists, index := function.IndexArrayWalk(dnsApi, func(i accessor.DnsApi) bool {
				return i.ServerName == item.ServerName
			}); exists {
				dnsApi[index] = item
			} else {
				dnsApi = append(dnsApi, item)
			}
		}
		for _, item := range params.User {
			if exists, index := function.IndexArrayWalk(dnsApi, func(i accessor.DnsApi) bool {
				return i.ServerName == item.ServerName
			}); exists {
				dnsApi[index] = item
			} else {
				dnsApi = append(dnsApi, item)
			}
		}
		err := logic2.Setting{}.Save(&entity.Setting{
			GroupName: logic2.SettingGroupSetting,
			Name:      logic2.SettingGroupSettingDnsApi,
			Value: &accessor.SettingValueOption{
				DnsApi: dnsApi,
			},
		})
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	dnsApi = append([]accessor.DnsApi{
		{
			ServerName: "nginx",
			Title:      "Nginx",
			Env:        make([]docker.EnvItem, 0),
		},
	}, dnsApi...)
	self.JsonResponseWithoutError(http, gin.H{
		"setting": dnsApi,
	})
	return
}
