package controller

import (
	"errors"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/exec"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"gorm.io/datatypes"
	"gorm.io/gen"
	"html/template"
	"log/slog"
	"os"
	"path/filepath"
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
		Id                        int32    `json:"id"`
		ContainerId               string   `json:"containerId"`
		ServerName                string   `json:"serverName" binding:"required"`
		ServerNameAlias           []string `json:"serverNameAlias"`
		Hostname                  string   `json:"hostname"`
		Port                      int32    `json:"port" binding:"required"`
		EnableBlockCommonExploits bool     `json:"enableBlockCommonExploits"`
		EnableAssetCache          bool     `json:"enableAssetCache"`
		EnableWs                  bool     `json:"enableWs"`
		ExtraNginx                string   `json:"extraNginx"`
		EnableSSL                 bool     `json:"enableSSL"`
		CertName                  string   `json:"certName"`
	}

	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	var err error
	var siteDomainRow *entity.SiteDomain

	if params.Id > 0 {
		if siteDomainRow, err = dao.SiteDomain.Where(dao.SiteDomain.ID.Eq(params.Id)).First(); err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		siteDomainRow.ContainerID = ""
	} else {
		siteDomainRow = &entity.SiteDomain{
			ServerName:  params.ServerName,
			ContainerID: "",
			Setting:     &accessor.SiteDomainSettingOption{},
		}
		if _, err = dao.SiteDomain.Where(dao.SiteDomain.ServerName.Eq(params.ServerName)).First(); err == nil {
			self.JsonResponseWithError(http, notice.Message{}.New(".siteDomainExists", "domain", params.ServerName), 500)
			return
		}
	}

	for _, alias := range params.ServerNameAlias {
		if item, err := dao.SiteDomain.
			Where(gen.Cond(datatypes.JSONQuery("setting").
				Likes("%"+alias+"%", "serverNameAlias"))...).
			First(); err == nil && (params.Id > 0 && params.Id != item.ID) {
			self.JsonResponseWithError(http, notice.Message{}.New(".siteDomainExists", "domain", alias), 500)
			return
		}
	}

	var hostname string
	if params.Hostname != "" {
		hostname = params.Hostname
	}
	if params.ContainerId != "" {
		containerRow, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, params.ContainerId)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		// 转发时必须保证当前环境有 dpanel 面板
		// 将当前容器加入到默认 dpanel-local 网络中，并指定 Hostname 用于 Nginx 反向代理
		dpanelContainerInfo := types.ContainerJSON{}
		if dpanelContainerInfo, err = docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, facade.GetConfig().GetString("app.name")); err != nil {
			self.JsonResponseWithError(http, notice.Message{}.New(".siteDomainNotFoundDPanel"), 500)
			return
		}
		slog.Debug("site domain dpanel container", "id", dpanelContainerInfo.ID)
		if _, err = docker.Sdk.Client.NetworkInspect(docker.Sdk.Ctx, defaultNetworkName, network.InspectOptions{}); err != nil {
			slog.Debug("site domain create default network", "name", defaultNetworkName)
			_, err = docker.Sdk.Client.NetworkCreate(docker.Sdk.Ctx, defaultNetworkName, network.CreateOptions{
				Driver: "bridge",
				Options: map[string]string{
					"name": defaultNetworkName,
				},
				EnableIPv6: function.PtrBool(false),
			})
			if err != nil {
				self.JsonResponseWithError(http, notice.Message{}.New(".siteDomainJoinDefaultNetworkFailed"), 500)
				return
			}
		}

		// 当面板自己没有加入默认网络时，加入并配置 hostname
		// 假如当前转发的容器就是面板自己，则不在这里处理，统一在下面加入网络
		if _, ok := dpanelContainerInfo.NetworkSettings.Networks[defaultNetworkName]; !ok {
			if dpanelContainerInfo.ID != containerRow.ID {
				err = docker.Sdk.Client.NetworkConnect(docker.Sdk.Ctx, defaultNetworkName, dpanelContainerInfo.ID, &network.EndpointSettings{
					Aliases: []string{
						fmt.Sprintf(docker.HostnameTemplate, strings.Trim(dpanelContainerInfo.Name, "/")),
					},
				})
				if err != nil {
					self.JsonResponseWithError(http, notice.Message{}.New(".siteDomainJoinDefaultNetworkFailed", err.Error()), 500)
					return
				}
			}
		}

		// 当目标容器不在默认网络时，加入默认网络
		hostname = fmt.Sprintf(docker.HostnameTemplate, strings.Trim(containerRow.Name, "/"))
		if _, ok := containerRow.NetworkSettings.Networks[defaultNetworkName]; !ok {
			slog.Debug("site domain join default network ", "container name", containerRow.Name, "hostname", hostname)
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
		siteDomainRow.ContainerID = containerRow.Name
	}

	siteDomainRow.Setting = &accessor.SiteDomainSettingOption{
		ServerName:                params.ServerName,
		ServerNameAlias:           params.ServerNameAlias,
		ServerAddress:             hostname,
		Port:                      params.Port,
		EnableBlockCommonExploits: params.EnableBlockCommonExploits,
		EnableWs:                  params.EnableWs,
		EnableAssetCache:          params.EnableAssetCache,
		EnableSSL:                 params.EnableSSL,
		ExtraNginx:                template.HTML(params.ExtraNginx),
		TargetName:                function.GetMd5(params.ServerName),
		CertName:                  "",
		SslCrt:                    "",
		SslKey:                    "",
	}

	if params.CertName != "" && params.EnableSSL {
		siteDomainRow.Setting.CertName = params.CertName
		siteDomainRow.Setting.SslCrt = filepath.Join(storage.Local{}.GetNginxCertPath(), fmt.Sprintf(logic.CertName, params.CertName), logic.CertFileName)
		siteDomainRow.Setting.SslKey = filepath.Join(storage.Local{}.GetNginxCertPath(), fmt.Sprintf(logic.CertName, params.CertName), fmt.Sprintf(logic.KeyFileName, params.CertName))
	}

	err = logic.Site{}.MakeNginxConf(siteDomainRow.Setting)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if siteDomainRow.ID > 0 {
		err = dao.SiteDomain.Save(siteDomainRow)
	} else {
		err = dao.SiteDomain.Create(siteDomainRow)
	}

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
		CertName    string `json:"certName"`
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
	if params.CertName != "" {
		query = query.Where(gen.Cond(datatypes.JSONQuery("setting").Equals(params.CertName, "certName"))...)
	}
	list, _ := query.Find()

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
	vhost, err := os.ReadFile(filepath.Join(storage.Local{}.GetNginxSettingPath(), fmt.Sprintf(logic.VhostFileName, domainRow.ServerName)))
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
		err := os.Remove(filepath.Join(storage.Local{}.GetNginxSettingPath(), fmt.Sprintf(logic.VhostFileName, item.ServerName)))
		if err != nil {
			slog.Debug("container delete domain", "error", err)
		}
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

func (self SiteDomain) RestartNginx(http *gin.Context) {
	cmd, _ := exec.New(
		exec.WithCommandName("nginx"),
		exec.WithArgs("-t"),
	)
	out := cmd.RunWithResult()
	slog.Debug("site domain restart nginx", "-t", out)
	if !strings.Contains(out, "successful") {
		self.JsonResponseWithError(http, errors.New(out), 500)
		return
	}
	cmd, _ = exec.New(
		exec.WithCommandName("nginx"),
		exec.WithArgs("-s", "reload"),
	)
	out = cmd.RunWithResult()
	slog.Debug("site domain stop nginx", "out", out)
	self.JsonSuccessResponse(http)
	return
}
