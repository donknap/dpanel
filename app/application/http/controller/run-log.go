package controller

import (
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"io"
	"strconv"
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
	out, err := docker.Sdk.Client.ContainerLogs(docker.Sdk.Ctx, params.Md5, container.LogsOptions{
		ShowStderr: true,
		ShowStdout: true,
		Tail:       strconv.Itoa(params.LineTotal),
		Follow:     true,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	myWrite := &write{http: http}
	//self.JsonResponseWithoutError(http, gin.H{
	//	"log": string(output),
	//})
	http.Header("Content-Type", "text/event-stream")
	http.SSEvent("data", "dafa")
	io.Copy(myWrite, out)
}
