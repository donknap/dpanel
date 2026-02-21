package controller

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/network"
	"github.com/donknap/dpanel/app/application/logic"
	logic2 "github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/exec/local"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"gorm.io/datatypes"
	"gorm.io/gen"
)

type SiteDomain struct {
	controller.Abstract
}

func (self SiteDomain) Create(http *gin.Context) {
	type ParamsValidate struct {
		Id          int32  `json:"id"`
		ContainerId string `json:"containerId"`
		accessor.SiteDomainSettingOption
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
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageSiteDomainExists, "domain", params.ServerName), 500)
			return
		}
	}

	for _, alias := range params.ServerNameAlias {
		if item, err := dao.SiteDomain.
			Where(gen.Cond(datatypes.JSONQuery("setting").
				Likes("%"+alias+"%", "serverNameAlias"))...).
			First(); err == nil && (params.Id > 0 && params.Id != item.ID) {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageSiteDomainExists, "domain", alias), 500)
			return
		}
	}

	if params.ContainerId != "" {
		containerRow, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, params.ContainerId)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		// 转发时必须保证当前环境有 dpanel 面板
		// 将当前容器加入到默认 dpanel-local 网络中，并指定 Hostname 用于 Nginx 反向代理
		dpanelInfo := logic2.Setting{}.GetDPanelInfo()
		if dpanelInfo.ContainerInfo.ContainerJSONBase == nil {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageSiteDomainNotFoundDPanel), 500)
			return
		}

		if _, err = docker.Sdk.Client.NetworkInspect(docker.Sdk.Ctx, define.DPanelProxyNetworkName, network.InspectOptions{}); err != nil {
			slog.Debug("site domain create default network", "name", define.DPanelProxyNetworkName)
			_, err = docker.Sdk.Client.NetworkCreate(docker.Sdk.Ctx, define.DPanelProxyNetworkName, network.CreateOptions{
				Driver: "bridge",
				Options: map[string]string{
					"name": define.DPanelProxyNetworkName,
				},
				EnableIPv6: function.Ptr(false),
			})
			if err != nil {
				self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageSiteDomainJoinDefaultNetworkFailed), 500)
				return
			}
		}

		dpanelLocalNetwork, err := docker.Sdk.Client.NetworkInspect(docker.Sdk.Ctx, define.DPanelProxyNetworkName, network.InspectOptions{})
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}

		// 当面板自己没有加入默认网络时，加入并配置 hostname
		// 假如当前转发的容器就是面板自己，则不在这里处理，统一在下面加入网络
		if _, _, ok := function.PluckMapItemWalk(dpanelLocalNetwork.Containers, func(k string, v network.EndpointResource) bool {
			return k == dpanelInfo.ContainerInfo.ID
		}); !ok {
			err = docker.Sdk.Client.NetworkConnect(docker.Sdk.Ctx, define.DPanelProxyNetworkName, dpanelInfo.ContainerInfo.ID, &network.EndpointSettings{
				Aliases: []string{
					fmt.Sprintf(define.DPanelNetworkHostName, strings.Trim(dpanelInfo.ContainerInfo.Name, "/")),
				},
			})
			if err != nil {
				self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageSiteDomainJoinDefaultNetworkFailed, err.Error()), 500)
				return
			}
		}

		// 当目标容器不在默认网络时，加入默认网络
		params.ServerAddress = fmt.Sprintf(define.DPanelNetworkHostName, strings.Trim(containerRow.Name, "/"))
		if _, ok := containerRow.NetworkSettings.Networks[define.DPanelProxyNetworkName]; !ok {
			slog.Debug("site domain join default network ", "container name", containerRow.Name, "hostname", params.ServerAddress)
			err = docker.Sdk.Client.NetworkConnect(docker.Sdk.Ctx, define.DPanelProxyNetworkName, params.ContainerId, &network.EndpointSettings{
				Aliases: []string{
					params.ServerAddress,
				},
			})
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
		}
		siteDomainRow.ContainerID = containerRow.Name
	}

	params.TargetName = function.Md5(params.ServerName)
	siteDomainRow.Setting = &params.SiteDomainSettingOption

	if params.CertName != "" && params.EnableSSL {
		siteDomainRow.Setting.CertName = params.CertName
		siteDomainRow.Setting.SslCrt = filepath.Join(storage.Local{}.GetCertDomainPath(), fmt.Sprintf(logic.CertName, params.CertName), logic.CertFileName)
		siteDomainRow.Setting.SslKey = filepath.Join(storage.Local{}.GetCertDomainPath(), fmt.Sprintf(logic.CertName, params.CertName), fmt.Sprintf(logic.KeyFileName, params.CertName))
	}

	err = logic.Site{}.MakeNginxConf(*siteDomainRow.Setting)
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
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}

	vhostFileName := fmt.Sprintf(logic.VhostFileName, domainRow.ServerName)
	vhost, err := os.ReadFile(filepath.Join(storage.Local{}.GetNginxSettingPath(), vhostFileName))
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if extraVhost, err := os.ReadFile(filepath.Join(storage.Local{}.GetNginxExtraSettingPath(), vhostFileName)); err == nil {
		domainRow.Setting.ExtraNginx = template.HTML(extraVhost)
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
			containerInfo, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, list[0].ContainerID)
			if err == nil && len(containerInfo.NetworkSettings.Networks) > 1 {
				err = docker.Sdk.Client.NetworkDisconnect(docker.Sdk.Ctx, define.DPanelProxyNetworkName, list[0].ContainerID, false)
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

func (self SiteDomain) NginxRestart(http *gin.Context) {
	if err := (logic.Site{}).MakeNginxResolver(); err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	out, err := local.QuickRun("nginx -t")
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if !strings.Contains(string(out), "successful") {
		self.JsonResponseWithError(http, errors.New(string(out)), 500)
		return
	}
	if b, _ := local.QuickCheckRunning("nginx"); b {
		_, err = local.QuickRun("nginx -s reload")
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	} else {
		// 尝试启动 nginx
		if cmd, err := local.New(
			local.WithCommandName("nginx"),
			local.WithArgs("-g", "daemon on;"),
		); err == nil {
			err = cmd.Run()
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
		}
	}
	self.JsonSuccessResponse(http)
	return
}

func (self SiteDomain) NginxLog(http *gin.Context) {
	type ParamsValidate struct {
		Log       []string `json:"log"`
		LineTotal int      `json:"lineTotal"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	wsBuffer := ws.NewProgressPip(ws.MessageTypeNginxLog)
	defer wsBuffer.Close()

	ctx, cancel := context.WithCancel(wsBuffer.Context())
	defer cancel()

	var wg sync.WaitGroup

	for _, s := range params.Log {
		wg.Add(1)

		go func(filename string) {
			defer wg.Done()

			file, err := os.Open(filename)
			if err != nil {
				wsBuffer.BroadcastMessage(function.ConsoleWriteError(fmt.Sprintf("打开文件失败 %s: %v", filename, err)))
				return
			}

			defer file.Close()

			err = function.FileSeekToLastNLines(file, params.LineTotal)
			if err != nil {
				wsBuffer.BroadcastMessage(function.ConsoleWriteError(err.Error()))
				return
			}

			reader := bufio.NewReader(file)
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				line, err := reader.ReadString('\n')
				if err != nil {
					if err == io.EOF {
						select {
						case <-ctx.Done():
							return
						case <-time.After(100 * time.Millisecond):
							continue
						}
					}
					wsBuffer.BroadcastMessage(function.ConsoleWriteError(err.Error()))
					return
				}
				wsBuffer.BroadcastMessage(line)
			}
		}(s)
	}

	go func() {
		wg.Wait()
		cancel()
		wsBuffer.Close()
	}()

	<-wsBuffer.Done()

	self.JsonSuccessResponse(http)
	return
}

func (self SiteDomain) UpdateVhost(http *gin.Context) {
	type ParamsValidate struct {
		Id    int32  `json:"id" binding:"required"`
		Vhost string `json:"vhost" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	siteDomainRow, err := dao.SiteDomain.Where(dao.SiteDomain.ID.Eq(params.Id)).First()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	nginxConfPath := filepath.Join(storage.Local{}.GetNginxSettingPath(), fmt.Sprintf(logic.VhostFileName, siteDomainRow.Setting.ServerName))
	err = os.WriteFile(nginxConfPath, []byte(params.Vhost), 0666)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	self.JsonSuccessResponse(http)
	return
}
