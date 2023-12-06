package controller

import (
	"errors"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
	"os"
	"strings"
)

type Image struct {
	controller.Abstract
}

func (self Image) CreateByDockerfile(http *gin.Context) {
	type ParamsValidate struct {
		DockerFile string   `form:"dockerFile" binding:"required"`
		Name       []string `form:"name[]" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	mustHasZipFile := false
	addStr := []string{
		"ADD",
		"COPY",
	}
	for _, str := range addStr {
		if strings.Contains(strings.ToUpper(params.DockerFile), str) {
			mustHasZipFile = true
		}
	}
	sdk, err := docker.NewDockerClient()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	builder := sdk.GetImageBuildBuilder()
	if mustHasZipFile {
		_, fileHeader, _ := http.Request.FormFile("zipFile")
		if fileHeader == nil {
			self.JsonResponseWithError(http, errors.New("Dockerfile中包含添加文件操作，请上传对应的zip包"), 500)
			return
		}
		file, _ := os.CreateTemp("", "dpanel")
		err = http.SaveUploadedFile(fileHeader, file.Name())
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		builder.WithDockerFileContent([]byte(params.DockerFile))
		builder.WithZipFilePath(file.Name())
		builder.Execute()
	} else {
		builder.WithDockerFileContent([]byte(params.DockerFile))
		builder.Execute()
	}
}
