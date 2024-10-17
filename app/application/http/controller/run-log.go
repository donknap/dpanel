package controller

import (
	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/common/function"
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
		Md5       string `json:"md5" binding:"required"`
		LineTotal int    `json:"lineTotal" binding:"required,number,oneof=50 100 200 500 1000"`
		Download  bool   `json:"download"`
	}

	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	out, err := docker.Sdk.Client.ContainerLogs(docker.Sdk.Ctx, params.Md5, container.LogsOptions{
		ShowStderr: true,
		ShowStdout: true,
		Tail:       strconv.Itoa(params.LineTotal),
		Follow:     false,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	output, _ := io.ReadAll(out)
	cleanOut := function.BytesCleanFunc([]rune(string(output)), func(b rune) bool {
		return b == '\ufffd'
	})
	if params.Download {
		http.Header("Content-Type", "text/plain")
		http.Header("Content-Disposition", "attachment; filename="+params.Md5+".log")
		http.Data(200, "text/plain", []byte(string(cleanOut)))
		return
	} else {
		self.JsonResponseWithoutError(http, gin.H{
			"log": string(cleanOut),
		})
	}
	return
}
