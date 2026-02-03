package controller

import (
	"net/url"
	"strings"

	"github.com/docker/docker/api/types/registry"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/donknap/dpanel/common/types/event"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type Registry struct {
	controller.Abstract
}

func (self Registry) Create(http *gin.Context) {
	type ParamsValidate struct {
		Id            int32    `json:"id"`
		Title         string   `json:"title" binding:"required"`
		Username      string   `json:"username"`
		Password      string   `json:"password"`
		ServerAddress string   `json:"serverAddress" binding:"required"`
		Email         string   `json:"email" binding:"omitempty"`
		Proxy         []string `json:"proxy"`
		EnableHttp    bool     `json:"enableHttp"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	var err error
	params.ServerAddress = strings.TrimPrefix(strings.TrimPrefix(params.ServerAddress, "https://"), "http://")
	urls, err := url.Parse("http://" + params.ServerAddress)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	params.ServerAddress = urls.Host
	var registryRow *entity.Registry
	if params.Id <= 0 {
		registryRow, _ = dao.Registry.Where(dao.Registry.ServerAddress.Eq(params.ServerAddress)).First()
		if registryRow != nil {
			// 类似腾讯云这样的仓库地址一样，如果用户名不一样也按两个仓库来对待
			if params.Username != "" && params.Password != "" && params.Username == registryRow.Setting.Username {
				self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonIdAlreadyExists, "name", params.ServerAddress), 500)
				return
			}
		}
	} else {
		registryRow, _ = dao.Registry.Where(dao.Registry.ID.Eq(params.Id)).First()
		if registryRow == nil {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
			return
		}
		// 如果提交上来密码为空，则使用默认密码
		if params.Password == "" && registryRow.Setting.Password != "" {
			code, _ := function.RSADecode(registryRow.Setting.Password, []byte(facade.GetConfig().GetString("app.name")))
			params.Password = code
		}
	}

	var response registry.AuthenticateOKBody
	authConfig := registry.AuthConfig{
		Username:      params.Username,
		Password:      params.Password,
		ServerAddress: params.ServerAddress,
		Email:         params.Email,
	}
	// 未设置用户名及密码时，这些匿名仓库不做登录操作，因为有可能无法访问
	if !function.InArray([]string{
		"docker.io",
		"quay.io",
		"ghcr.io",
	}, params.ServerAddress) || params.Username != "" {
		response, err = docker.Sdk.Client.RegistryLogin(docker.Sdk.Ctx, authConfig)
	}

	if err != nil {
		if function.ErrorHasKeyword(err, "server gave HTTP response to HTTPS client") {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageImagePullServerHttp, "name", params.ServerAddress), 500)
			return
		}
		self.JsonResponseWithError(http, err, 500)
		return
	}

	registryNew := &entity.Registry{
		Title:         params.Title,
		ServerAddress: params.ServerAddress,
		Setting: &accessor.RegistrySettingOption{
			Username:   params.Username,
			Email:      params.Email,
			Proxy:      params.Proxy,
			Password:   "",
			EnableHttp: params.EnableHttp,
		},
	}
	if params.Password != "" {
		code, _ := function.RSAEncode(params.Password)
		registryNew.Setting.Password = code
	}

	if params.Id <= 0 {
		err = dao.Registry.Create(registryNew)
	} else {
		_, err = dao.Registry.Where(dao.Registry.ID.Eq(params.Id)).Updates(registryNew)
		registryNew.ID = params.Id
	}

	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	if params.Id <= 0 {
		facade.GetEvent().Publish(event.ImageRegistryCreateEvent, event.ImageRegistryPayload{
			Registry: registryNew,
			Ctx:      http,
		})
	} else {
		facade.GetEvent().Publish(event.ImageRegistryEditEvent, event.ImageRegistryPayload{
			Registry:    registryNew,
			OldRegistry: registryRow,
			Ctx:         http,
		})
	}

	self.JsonResponseWithoutError(http, gin.H{
		"status": response.Status,
		"id":     registryNew.ID,
	})
	return
}

func (self Registry) GetList(http *gin.Context) {
	type ParamsValidate struct {
		Title         string `json:"title"`
		ServerAddress string `json:"serverAddress"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	var list []*entity.Registry

	query := dao.Registry.Order(dao.Registry.ID.Desc())
	if params.Title != "" {
		query = query.Where(dao.Registry.Title.Like("%" + params.Title + "%"))
	}
	if params.ServerAddress != "" {
		query = query.Where(dao.Registry.ServerAddress.Like("%" + params.ServerAddress + "%"))
	}
	list, _ = query.Find()
	for index, _ := range list {
		list[index].Setting.Password = "****"
	}
	self.JsonResponseWithoutError(http, gin.H{
		"list": list,
	})
	return
}

func (self Registry) GetDetail(http *gin.Context) {
	type ParamsValidate struct {
		Id int32 `json:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	registryItem, _ := dao.Registry.Where(dao.Registry.ID.Eq(params.Id)).First()
	if registryItem == nil {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}
	if registryItem.Setting != nil && registryItem.Setting.Password != "" {
		registryItem.Setting.Password, _ = function.RSADecode(registryItem.Setting.Password, []byte(facade.GetConfig().GetString("app.name")))
	}
	self.JsonResponseWithoutError(http, gin.H{
		"info": registryItem,
	})
	return
}

func (self Registry) Delete(http *gin.Context) {
	type ParamsValidate struct {
		Id []int32 `json:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	rows, _ := dao.Registry.Where(dao.Registry.ID.In(params.Id...)).Find()
	if rows == nil || len(rows) == 0 {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}

	_, err := dao.Registry.Where(dao.Registry.ID.In(params.Id...)).Delete()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	delServerAddress := make([]string, 0)
	for _, item := range rows {
		delServerAddress = append(delServerAddress, item.ServerAddress)

		facade.GetEvent().Publish(event.ImageRegistryDeleteEvent, event.ImageRegistryPayload{
			Registry: item,
			Ctx:      http,
		})
	}

	self.JsonSuccessResponse(http)
	return
}
