package controller

import (
	"errors"
	"github.com/docker/docker/api/types"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go-support/src/facade"
	"strings"
)

func (self Image) TagRemote(http *gin.Context) {
	type ParamsValidate struct {
		Tag      string `json:"tag" binding:"required"`
		Type     string `json:"type" binding:"required,oneof=pull push"`
		AsLatest bool   `json:"asLatest"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	var authString string
	tagArr := strings.Split(params.Tag, "/")
	registry, _ := dao.Registry.Where(dao.Registry.ServerAddress.Eq(tagArr[0])).First()
	if registry != nil {
		password, _ := function.AseDecode(facade.GetConfig().GetString("app.name"), registry.Password)
		authString = function.Base64Encode(struct {
			Username      string `json:"username"`
			Password      string `json:"password"`
			ServerAddress string `json:"serveraddress"`
		}{
			Username:      registry.Username,
			Password:      password,
			ServerAddress: registry.ServerAddress,
		})
	}
	if params.AsLatest {
		tag := strings.Split(params.Tag, ":")
		latestTag := tag[0] + ":latest"
		err := docker.Sdk.Client.ImageTag(docker.Sdk.Ctx, params.Tag, latestTag)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		err = logic.DockerTask{}.ImageRemote(&logic.ImageRemoteMessage{
			Auth: authString,
			Type: params.Type,
			Tag:  latestTag,
		})
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	} else {
		err := logic.DockerTask{}.ImageRemote(&logic.ImageRemoteMessage{
			Auth: authString,
			Type: params.Type,
			Tag:  params.Tag,
		})
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	self.JsonResponseWithoutError(http, gin.H{
		"tag": params.Tag,
	})
	return
}

func (self Image) TagDelete(http *gin.Context) {
	type ParamsValidate struct {
		Tag   string `form:"tag" binding:"required"`
		Force bool   `form:"force" binding:"omitempty"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	_, err := docker.Sdk.Client.ImageRemove(docker.Sdk.Ctx, params.Tag, types.ImageRemoveOptions{
		Force: params.Force,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"tag": params.Tag,
	})
	return
}

func (self Image) TagAdd(http *gin.Context) {
	type ParamsValidate struct {
		Md5 string `form:"md5" binding:"required"`
		Tag string `form:"tag" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	imageDetail, _, err := docker.Sdk.Client.ImageInspectWithRaw(docker.Sdk.Ctx, params.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if function.InArray[string](imageDetail.RepoTags, params.Tag) {
		self.JsonResponseWithError(http, errors.New("该标签已经存在"), 500)
		return
	}
	err = docker.Sdk.Client.ImageTag(docker.Sdk.Ctx, imageDetail.ID, params.Tag)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"tag": params.Tag,
	})
	return
}
