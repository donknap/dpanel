package controller

import (
	"context"
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
		Title         string `form:"title" binding:"required"`
		Username      string `form:"username" binding:"required"`
		Password      string `form:"password" binding:"required"`
		ServerAddress string `form:"serverAddress" binding:"required"`
		Email         string `form:"email" binding:"omitempty"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	registryRow, _ := dao.Registry.Where(dao.Registry.ServerAddress.Eq(params.ServerAddress)).First()
	if registryRow != nil {
		self.JsonResponseWithError(http, errors.New("仓库已经存在"), 500)
		return
	}
	sdk, err := docker.NewDockerClient()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	response, err := sdk.Client.RegistryLogin(context.Background(), registry.AuthConfig{
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
	dao.Registry.Create(registryNew)

	self.JsonResponseWithoutError(http, gin.H{
		"status": response.Status,
		"id":     registryNew.ID,
	})
	return
}

func (self Registry) GetList(http *gin.Context) {
	hasDockerIo := false
	var list []*entity.Registry
	list, _ = dao.Registry.Select(dao.Registry.Title, dao.Registry.ServerAddress, dao.Registry.ID).Find()
	for _, item := range list {
		if item.ServerAddress == "docker.io" {
			hasDockerIo = true
			break
		}
	}
	if !hasDockerIo {
		list = append(list, &entity.Registry{
			Title:         "Docker Hub",
			ServerAddress: "docker.io",
			Username:      "anonymous",
		})
	}
	self.JsonResponseWithoutError(http, gin.H{
		"list": list,
	})
	return
}
