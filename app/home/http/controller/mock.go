package controller

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
)

type Mock struct {
	controller.Abstract
}

func (self Mock) UserInfo(http *gin.Context) {
	self.JsonResponseWithoutError(http, gin.H{
		"name":   "test",
		"avatar": "abc",
	})
	return
}

func (self Mock) Error(http *gin.Context) {
	self.JsonResponseWithError(http, errors.New("错了"), 500)
	return
}
