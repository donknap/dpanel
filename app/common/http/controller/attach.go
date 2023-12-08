package controller

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
	"os"
	"path/filepath"
)

type Attach struct {
	controller.Abstract
}

func (self Attach) Upload(http *gin.Context) {
	fileUploader, fileHeader, _ := http.Request.FormFile("file")
	if fileHeader == nil {
		self.JsonResponseWithError(http, errors.New("请指定上传文件"), 500)
		return
	}
	defer fileUploader.Close()
	file, _ := os.CreateTemp("", "dpanel-upload")
	err := http.SaveUploadedFile(fileHeader, file.Name())
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"path": filepath.Base(file.Name()),
	})
	return
}
