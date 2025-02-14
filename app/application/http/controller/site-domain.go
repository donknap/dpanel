package controller

import (
	"errors"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	"github.com/donknap/dpanel/app/application/logic"
	logic2 "github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/exec"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
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
	} else {
		siteDomainRow = &entity.SiteDomain{
			ServerName:  params.ServerName,
			ContainerID: "",
			Setting:     &accessor.SiteDomainSettingOption{},
		}
		if _, err = dao.SiteDomain.Where(dao.SiteDomain.ServerName.Eq(params.ServerName)).First(); err == nil {
			self.JsonResponseWithError(http, errors.New("域名已经存在"), 500)
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
		// 将当前容器加入到默认 dpanel-local 网络中，并指定 Hostname 用于 Nginx 反向代理
		dpanelContainerInfo := types.ContainerJSON{}
		if exists := new(logic2.Setting).GetByKey(logic2.SettingGroupSetting, logic2.SettingGroupSettingDPanelInfo, &dpanelContainerInfo); !exists {
			self.JsonResponseWithError(http, errors.New(".siteDomainNotFoundDPanel"), 500)
			return
		}
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
					self.JsonResponseWithError(http, errors.New(".siteDomainJoinDefaultNetworkFailed"), 500)
					return
				}
			}
			// 假如是自身绑定域名，不加入网络，在下面统一处理
			if dpanelContainerInfo.ID != containerRow.ID {
				err = docker.Sdk.Client.NetworkConnect(docker.Sdk.Ctx, defaultNetworkName, dpanelContainerInfo.ID, &network.EndpointSettings{})
				if err != nil {
					self.JsonResponseWithError(http, errors.New(".siteDomainJoinDefaultNetworkFailed"), 500)
					return
				}
			}
		}
		hostname = fmt.Sprintf(docker.HostnameTemplate, strings.Trim(containerRow.Name, "/"))
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
		siteDomainRow.ContainerID = containerRow.ID
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
		siteDomainRow.Setting.CertName = fmt.Sprintf(logic.CertName, params.CertName)
		siteDomainRow.Setting.SslCrt = filepath.Join(storage.Local{}.GetNginxCertPath(), siteDomainRow.Setting.CertName, logic.CertFileName)
		siteDomainRow.Setting.SslKey = filepath.Join(storage.Local{}.GetNginxCertPath(), siteDomainRow.Setting.CertName, fmt.Sprintf(logic.KeyFileName, params.CertName))
	}

	err = logic.Site{}.MakeNginxConf(siteDomainRow.Setting)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if siteDomainRow.ID > 0 {
		_, err = dao.SiteDomain.Updates(siteDomainRow)
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
		go func() {
			err := os.Remove(filepath.Join(storage.Local{}.GetNginxSettingPath(), fmt.Sprintf(logic.VhostFileName, item.ServerName)))
			if err != nil {
				slog.Debug("container delete domain", "error", err)
			}
		}()
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
