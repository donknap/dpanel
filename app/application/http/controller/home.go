package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type Home struct {
	controller.Abstract
}

func (home Home) Index(ctx *gin.Context) {
	home.JsonResponseWithoutError(ctx, "hello world!")
}
