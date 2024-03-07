package controller

import (
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
	"io"
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

	statRow, err := docker.Sdk.Client.ContainerStats(docker.Sdk.Ctx, params.Id, false)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	statInfo, err := io.ReadAll(statRow.Body)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer statRow.Body.Close()
	http.Header("Content-type", "application/json;charset=utf-8")
	http.String(http2.StatusOK, string(statInfo))
	return

}
