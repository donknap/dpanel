package controller

import (
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
	"io"
	"strconv"
)

type RunLog struct {
	controller.Abstract
}

type write struct {
	http *gin.Context
}

func (self *write) Write(p []byte) (n int, err error) {
	self.http.SSEvent("log", string(p))
	fmt.Printf("%v \n", string(p))
	return len(p), nil
}

func (self RunLog) Run(http *gin.Context) {
	type ParamsValidate struct {
		Md5       string `json:"md5" binding:"required"`
		LineTotal int    `json:"lineTotal" binding:"required,number,oneof=50 100 200 500 1000"`
	}
	//content1, _:=io.ReadAll(http.Request.Body)
	//fmt.Printf("%v \n", string(content1))
	params := ParamsValidate{
		Md5:       "5eefb640f145c43ef985fbbfd1011364aee2354b57141b6242d6bdf36fb98c8a",
		LineTotal: 50,
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
