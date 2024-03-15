package controller

import (
	"database/sql/driver"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
	"strings"
)

type Container struct {
	controller.Abstract
}

func (self Container) Status(http *gin.Context) {
	type ParamsValidate struct {
		Md5     string `form:"md5" binding:"required"`
		Operate string `form:"operate" binding:"required,oneof=start stop restart pause unpause"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	var err error
	switch params.Operate {
	case "restart":
		err = docker.Sdk.Client.ContainerRestart(docker.Sdk.Ctx,
			params.Md5,
			container.StopOptions{})
	case "stop":
		err = docker.Sdk.Client.ContainerStop(docker.Sdk.Ctx,
			params.Md5,
			container.StopOptions{})
	case "start":
		err = docker.Sdk.Client.ContainerStart(docker.Sdk.Ctx,
			params.Md5,
			container.StartOptions{})
	case "pause":
		err = docker.Sdk.Client.ContainerPause(docker.Sdk.Ctx,
			params.Md5)
	case "unpause":
		err = docker.Sdk.Client.ContainerUnpause(docker.Sdk.Ctx,
			params.Md5)
	}
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Container) GetList(http *gin.Context) {
	type ParamsValidate struct {
		Tag       string `json:"tag"`
		SiteTitle string `json:"siteTitle"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	var list []types.Container
	filter := filters.NewArgs()
	if params.Tag != "" {
		filter.Add("name", params.Tag)
	}
	list, err := docker.Sdk.Client.ContainerList(docker.Sdk.Ctx, container.ListOptions{
		All:     true,
		Latest:  true,
		Filters: filter,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	if function.IsEmptyArray(list) {
		list = make([]types.Container, 0)
	} else {
		var md5List []driver.Valuer
		for _, item := range list {
			md5List = append(md5List, &accessor.SiteContainerInfoOption{
				ID: item.ID,
			})
		}
		siteList, _ := dao.Site.Select(
			dao.Site.SiteTitle,
			dao.Site.SiteName,
			dao.Site.ID,
		).Where(dao.Site.ContainerInfo.In(md5List...)).Find()

		for i, item := range list {
			tagName := item.Names[0]
			for _, name := range item.Names {
				if strings.Count(name, "/") == 1 {
					tagName = name
					break
				}
			}
			has := false
			for _, site := range siteList {
				if function.InArray(item.Names, "/"+site.SiteName) {
					list[i].Names = []string{
						tagName,
						site.SiteTitle,
					}
					has = true
					break
				}
			}
			if !has {
				list[i].Names = []string{
					tagName,
				}
			}
		}

		if params.SiteTitle != "" {
			temp := make([]types.Container, 0)
			for _, item := range list {
				if len(item.Names) > 1 && strings.Contains(item.Names[1], params.SiteTitle) {
					temp = append(temp, item)
				}
			}
			self.JsonResponseWithoutError(http, gin.H{
				"list": temp,
			})
		} else {
			self.JsonResponseWithoutError(http, gin.H{
				"list": list,
			})
		}
	}
	return
}

func (self Container) GetDetail(http *gin.Context) {
	type ParamsValidate struct {
		Md5 string `form:"md5" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	detail, err := docker.Sdk.ContainerInfo(params.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"info": detail,
	})
	return
}

func (self Container) Update(http *gin.Context) {
	type ParamsValidate struct {
		Md5     string `json:"md5" binding:"required"`
		Restart string `json:"restart" binding:"omitempty,oneof=no on-failure unless-stopped always"`
		Name    string `json:"name"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if params.Restart != "" {
		restartPolicy := container.RestartPolicy{
			Name: docker.Sdk.GetRestartPolicyByString(params.Restart),
		}
		if params.Restart == "on-failure" {
			restartPolicy.MaximumRetryCount = 5
		}
		_, err := docker.Sdk.Client.ContainerUpdate(docker.Sdk.Ctx, params.Md5, container.UpdateConfig{
			RestartPolicy: restartPolicy,
		})
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}

	}
	if params.Name != "" {
		err := docker.Sdk.Client.ContainerRename(docker.Sdk.Ctx, params.Md5, params.Name)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Container) Prune(http *gin.Context) {
	filter := filters.NewArgs()
	docker.Sdk.Client.ContainersPrune(docker.Sdk.Ctx, filter)
	self.JsonSuccessResponse(http)
	return
}
