package controller

import "github.com/gin-gonic/gin"

func (self Image) CreateByBuildX(http *gin.Context) {
	type ParamsValidate struct {
		Id             int32    `json:"id" binding:"required"`
		Platform       []string `json:"platform" binding:"required"`
		ClearContainer bool     `json:"clearContainer" binding:"required"`
		ClearCache     bool     `json:"clearCache" binding:"required"`
		Registry       string   `json:"registry"`
	}

	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

}
