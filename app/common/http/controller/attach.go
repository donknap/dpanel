package controller

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/donknap/dpanel/app/common/logic"
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
	defer func() {
		_ = file.Close()
	}()
	slog.Debug("upload file", "path", file.Name())
	err = http.SaveUploadedFile(fileHeader, file.Name())
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	rel, err := filepath.Rel(storage.Local{}.GetSaveRootPath(), file.Name())
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"path": rel,
	})
	return
}

func (self Attach) Delete(http *gin.Context) {
	type ParamsValidate struct {
		Path string `json:"path" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if !filepath.IsLocal(params.Path) {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}
	params.Path = function.PathClean(params.Path)
	uploadFile, err := storage.Local{}.CreateTempFile(params.Path)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	path := uploadFile.Name()
	err = uploadFile.Close()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	_, err = os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	err = os.Remove(path)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Attach) Download(http *gin.Context) {
	type ParamsValidate struct {
		Id          string `json:"id"`
		ContentType string `json:"contentType"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	params.Id = http.Query("id")

	cacheKey := fmt.Sprintf(storage.CacheKeyAttach, params.Id)
	val, ok := storage.Cache.Get(cacheKey)
	if !ok {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 404)
		return
	}

	task, ok := val.(*logic.AttachDownloadTask)
	if !ok {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}
	http.FileAttachment(task.FilePath, filepath.Base(task.FilePath))
	if task.DeleteAfterDownload {
		storage.Cache.Delete(cacheKey)
		if err := function.SafeDelete(storage.Local{}.GetSaveRootPath(), task.FilePath); err != nil {
			slog.Warn("delete attach download file after download failed", "path", task.FilePath, "error", err)
		}
	}
	return
}
