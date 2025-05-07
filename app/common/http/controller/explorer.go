package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type Explorer struct {
	controller.Abstract
}

func (self Explorer) GetPathList(http *gin.Context) {
	type ParamsValidate struct {
		Md5  string `json:"md5" binding:"required"`
		Path string `json:"path" binding:"required"`
	}

}
