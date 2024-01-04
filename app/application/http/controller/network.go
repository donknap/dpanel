package controller

import (
	"github.com/docker/docker/api/types"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
)

type Network struct {
	controller.Abstract
}

func (self Network) GetDetail(http *gin.Context) {
	type ParamsValidate struct {
		Name string `json:"name" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	networkInfo, _, err := docker.Sdk.Client.NetworkInspectWithRaw(docker.Sdk.Ctx, params.Name, types.NetworkInspectOptions{})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"info": networkInfo,
	})
	return

}
