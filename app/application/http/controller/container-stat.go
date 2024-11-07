package controller

import (
	"bytes"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
	"io"
	"log/slog"
	http2 "net/http"
)

func (self Container) GetStatInfo(http *gin.Context) {
	type ParamsValidate struct {
		Id string `json:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	out, err := docker.Sdk.Client.ContainerStats(docker.Sdk.Ctx, params.Id, false)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		if out.Body.Close() != nil {
			slog.Error("container", "stat close", err)
		}
	}()
	buffer := new(bytes.Buffer)
	_, err = io.Copy(buffer, out.Body)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	http.Header("Content-type", "application/json;charset=utf-8")
	http.String(http2.StatusOK, buffer.String())
	return

}

func (self Container) GetProcessInfo(http *gin.Context) {
	type ParamsValidate struct {
		Id string `json:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	psInfo, err := docker.Sdk.Client.ContainerTop(docker.Sdk.Ctx, params.Id, nil)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"list": psInfo,
	})
	return
}
