package controller

import (
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/gin-gonic/gin"
)

func (self Image) GetTemplateList(http *gin.Context) {
	self.JsonResponseWithoutError(http, logic.ImageTemplate{}.GetSupportEnv())
	return
}

func (self Image) GetTemplateDockerfile(http *gin.Context) {
	type ParamsValidate struct {
		Language string `json:"language" binding:"required"`
		Version  string `json:"version" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

}
