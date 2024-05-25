package controller

import (
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
)

type RunLog struct {
	controller.Abstract
}

func (self RunLog) Run(http *gin.Context) {
	type ParamsValidate struct {
		Md5       string `form:"md5" binding:"required"`
		LineTotal int    `form:"lineTotal" binding:"required,number,oneof=50 100 200 500 1000"`
	}

	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	builder := docker.Sdk.GetContainerLogBuilder()
	builder.WithContainerId(params.Md5)
	builder.WithTail(params.LineTotal)
	content, err := builder.Execute()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"log": content,
	})
	return
}
