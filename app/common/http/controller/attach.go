package controller

import (
	"errors"
	"fmt"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"log/slog"
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
	file, err := os.CreateTemp(storage.Local{}.GetSaveRootPath(), "dpanel-upload")
	if file != nil {
		slog.Debug("upload file", "path", file.Name())
	}
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	err = http.SaveUploadedFile(fileHeader, file.Name())
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"path": filepath.Base(file.Name()),
	})
	return
}

func (self Attach) Delete(http *gin.Context) {
	type ParamsValidate struct {
		Path string `form:"path" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	path := storage.Local{}.GetRealPath(params.Path)
	fmt.Printf("%v \n", path)
	_, err := os.Stat(path)
	if err == nil {
		os.Remove(path)
	}
	self.JsonSuccessResponse(http)
	return
}
