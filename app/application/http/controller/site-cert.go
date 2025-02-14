package controller

import (
	"errors"
	"fmt"
	logic2 "github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/acme"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"io"
	"strings"
)

type SiteCert struct {
	controller.Abstract
}

func (self SiteCert) DnsApi(http *gin.Context) {
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

func (self SiteCert) Apply(http *gin.Context) {
	type ParamsValidate struct {
		Domain      []string `json:"domain" binding:"required"`
		Email       string   `json:"email" binding:"required"`
		CertServer  string   `json:"certServer" binding:"required" oneof:"zerossl letsencrypt"`
		AutoUpgrade bool     `json:"autoUpgrade"`
		Renew       bool     `json:"renew"`
		Debug       bool     `json:"debug"`
		DnsApi      string   `json:"dnsApi"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	options := []acme.Option{
		acme.WithDomain(params.Domain...),
		acme.WithEmail(params.Email),
		acme.WithCertServer(params.CertServer),
		//acme.WithCertRootPath(storage.Local{}.GetStorageCertPath()),
	}

	if params.AutoUpgrade {
		options = append(options, acme.WithAutoUpgrade())
	}

	if params.Renew {
		options = append(options, acme.WithRenew(), acme.WithForce())
	} else {
		options = append(options, acme.WithIssue(), acme.WithForce())
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
				env := function.PluckArrayWalk(dnsApiList[i].Env, func(i docker.EnvItem) (string, bool) {
					return fmt.Sprintf("%s=%s", i.Name, i.Value), true
				})
				options = append(options, acme.WithDnsApi(dnsApiList[i].ServerName, env))
			}
		}
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

	success := false
	wsBuffer.OnWrite = func(p string) error {
		wsBuffer.BroadcastMessage(p)
		if strings.Contains(p, "-----END CERTIFICATE-----") {
			success = true
		}
		return nil
	}

	_, err = io.Copy(wsBuffer, response)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if success {
		self.JsonSuccessResponse(http)
	} else {
		self.JsonResponseWithError(http, errors.New(".domainCertIssueFailed"), 500)
	}
	return
}

func (self SiteCert) GetList(http *gin.Context) {
	builder, err := acme.New()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	list, err := builder.List()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"list": list,
	})
	return
}
