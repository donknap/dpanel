package controller

import (
	"errors"
	"github.com/docker/docker/api/types/registry"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go-support/src/facade"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
)

type Registry struct {
	controller.Abstract
}

func (self Registry) Create(http *gin.Context) {
	type ParamsValidate struct {
		Id            int32  `json:"id"`
		Title         string `form:"title" binding:"required"`
		Username      string `form:"username"`
		Password      string `form:"password"`
		ServerAddress string `form:"serverAddress"`
		Email         string `form:"email" binding:"omitempty"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	if params.Id <= 0 {
		registryRow, _ := dao.Registry.Where(dao.Registry.ServerAddress.Eq(params.ServerAddress)).First()
		if registryRow != nil {
			self.JsonResponseWithError(http, errors.New("仓库已经存在"), 500)
			return
		}
	} else {
		registryRow, _ := dao.Registry.Where(dao.Registry.ID.Eq(params.Id)).First()
		if registryRow == nil {
			self.JsonResponseWithError(http, errors.New("仓库不存在"), 500)
			return
		}
		// 如果提交上来密码为空，则使用默认密码
		if params.Password == "" {
			code, _ := function.AseDecode(facade.GetConfig().GetString("app.name"), registryRow.Password)
			params.Password = code
		}
	}

	response, err := docker.Sdk.Client.RegistryLogin(docker.Sdk.Ctx, registry.AuthConfig{
		Username:      params.Username,
		Password:      params.Password,
		ServerAddress: params.ServerAddress,
		Email:         params.Email,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	registryNew := &entity.Registry{
		Title:         params.Title,
		Username:      params.Username,
		ServerAddress: params.ServerAddress,
		Email:         params.Email,
	}
	key := facade.GetConfig().GetString("app.name")
	code, _ := function.AseEncode(key, params.Password)
	registryNew.Password = code

	if params.Id <= 0 {
		dao.Registry.Create(registryNew)
	} else {
		dao.Registry.Where(dao.Registry.ID.Eq(params.Id)).Updates(registryNew)
		registryNew.ID = params.Id
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

	hasDockerIo := false
	var list []*entity.Registry

	query := dao.Registry.Order(dao.Registry.ID.Desc())
	if params.Title != "" {
		query = query.Where(dao.Registry.Title.Like("%" + params.Title + "%"))
	}
	if params.ServerAddress != "" {
		query = query.Where(dao.Registry.ServerAddress.Like("%" + params.ServerAddress + "%"))
	}
	list, _ = query.Select(
		dao.Registry.Title,
		dao.Registry.ServerAddress,
		dao.Registry.ID,
		dao.Registry.Username,
	).Find()
	for _, item := range list {
		if item.ServerAddress == "docker.io" {
			hasDockerIo = true
			break
		}
	}
	if !hasDockerIo && params.Title == "" && params.ServerAddress == "" {
		list = append([]*entity.Registry{
			{
				Title:         "Docker Hub",
				ServerAddress: "docker.io",
				Username:      "anonymous",
			},
		}, list...)
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
		self.JsonResponseWithError(http, errors.New("仓库不存在"), 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"info": registryItem,
	})
	return
}

func (self Registry) Update(http *gin.Context) {
	type ParamsValidate struct {
		Id            int32  `json:"id" binding:"required"`
		Title         string `json:"title"`
		ServerAddress string `json:"serverAddress"`
		Username      string `json:"username"`
		Password      string `json:"password"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	row, _ := dao.Registry.Where(dao.Registry.ID.Eq(params.Id)).First()
	if row == nil {
		self.JsonResponseWithError(http, errors.New("仓库不存在"), 500)
		return
	}
	password := row.Password
	if params.Password != "" {
		password, _ = function.AseEncode(facade.GetConfig().GetString("app.name"), params.Password)
	}
	_, err := dao.Registry.Where(dao.Registry.ID.Eq(params.Id)).Updates(&entity.Registry{
		Title:         params.Title,
		ServerAddress: params.ServerAddress,
		Username:      params.Username,
		Password:      password,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"id": params.Id,
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
	_, err := dao.Registry.Where(dao.Registry.ID.In(params.Id...)).Delete()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}
