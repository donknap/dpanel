package controller

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type Attach struct {
	controller.Abstract
}

func (self Attach) Upload(http *gin.Context) {
	fileUploader, fileHeader, _ := http.Request.FormFile("file")
	if fileHeader == nil {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonUploadFileEmpty), 500)
		return
	}
	defer func() {
		_ = fileUploader.Close()
	}()
	file, err := storage.Local{}.CreateTempFile("")
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	slog.Debug("upload file", "path", file.Name())
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
	path := storage.Local{}.GetSaveRealPath(params.Path)
	fmt.Printf("%v \n", path)
	_, err := os.Stat(path)
	if err == nil {
		os.Remove(path)
	}
	self.JsonSuccessResponse(http)
	return
}
