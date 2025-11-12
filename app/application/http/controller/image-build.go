package controller

import (
	"fmt"
	"os"

	"github.com/docker/docker/api/types/build"
	"github.com/docker/go-units"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type ImageBuild struct {
	controller.Abstract
}

func (self ImageBuild) Create(http *gin.Context) {
	type ParamsValidate struct {
		Id    int32  `json:"id"`
		Title string `json:"title"`
		accessor.ImageSettingOption
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if params.BuildDockerfileContent == "" && params.BuildZip == "" && params.BuildGit == "" {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageImageBuildTypeEmpty), 500)
		return
	}
	if params.BuildZip != "" && params.BuildGit != "" {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageImageBuildTypeConflict), 500)
		return
	}
	imageNameDetail := function.ImageTag(params.Tag)
	if params.Registry != "" {
		imageNameDetail.Registry = params.Registry
	}
	params.Tag = imageNameDetail.Uri()

	if params.BuildZip != "" {
		path := storage.Local{}.GetSaveRealPath(params.BuildZip)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonUploadFileEmpty), 500)
			return
		}
		params.BuildZip = path
	}

	imageNew := &entity.Image{
		Tag:       "",
		BuildType: "",
		Title:     params.Title,
		Setting:   &params.ImageSettingOption,
		Status:    docker.ImageBuildStatusStop,
		Message:   "",
	}
	if imageRow, _ := dao.Image.Where(dao.Image.ID.Eq(params.Id)).First(); imageRow != nil {
		imageNew.ID = imageRow.ID
	}
	_ = dao.Image.Save(imageNew)

	log, err := logic.DockerTask{}.ImageBuild(fmt.Sprintf(ws.MessageTypeImageBuild, params.Id), params.ImageSettingOption)
	if err != nil {
		imageNew.Status = docker.ImageBuildStatusError
		imageNew.Message = log + "\n" + err.Error()
		_ = dao.Image.Save(imageNew)
		self.JsonResponseWithError(http, err, 500)
		return
	}
	imageNew.Status = docker.ImageBuildStatusSuccess
	imageNew.Message = log
	_ = dao.Image.Save(imageNew)
	self.JsonResponseWithoutError(http, gin.H{
		"imageId": imageNew.ID,
	})
	return
}

func (self ImageBuild) GetDetail(http *gin.Context) {
	type ParamsValidate struct {
		Id int32 `form:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	imageRow, _ := dao.Image.Where(dao.Image.ID.Eq(params.Id)).First()
	if imageRow == nil {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}
	tag := imageRow.Setting.Tag
	if tag == "" {
		tag = imageRow.Tag
	}
	tagDetail := function.ImageTag(tag)
	imageRow.Setting.Tag = tagDetail.BaseName
	if imageRow.Setting.BuildType == "" {
		imageRow.Setting.BuildType = imageRow.BuildType
	}
	if imageRow.Setting.BuildDockerfileContent == "" {
		imageRow.Setting.BuildDockerfileContent = imageRow.Setting.BuildDockerfile
	}
	if imageRow.Setting.BuildDockerfileRoot == "" {
		imageRow.Setting.BuildDockerfileRoot = imageRow.Setting.BuildRoot
	}
	self.JsonResponseWithoutError(http, gin.H{
		"detail": imageRow,
	})
	return
}

func (self ImageBuild) Delete(http *gin.Context) {
	type ParamsValidate struct {
		Id []int32 `form:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	_, err := dao.Image.Where(dao.Image.ID.In(params.Id...)).Delete()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}

func (self ImageBuild) GetList(http *gin.Context) {
	list, err := dao.Image.Order(dao.Image.ID.Desc()).Where(dao.Image.Setting.IsNotNull()).Find()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	list = function.PluckArrayWalk(list, func(item *entity.Image) (*entity.Image, bool) {
		if imageInfo, err := docker.Sdk.Client.ImageInspect(docker.Sdk.Ctx, item.Setting.Tag); err == nil {
			item.Setting.ImageId = imageInfo.ID
		}
		return item, true
	})
	self.JsonResponseWithoutError(http, gin.H{
		"list": list,
	})
	return
}

func (self ImageBuild) Prune(http *gin.Context) {
	res, err := docker.Sdk.Client.BuildCachePrune(docker.Sdk.Ctx, build.CachePruneOptions{
		All: true,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	_ = notice.Message{}.Info(".imageBuildPrune", "size", units.HumanSize(float64(res.SpaceReclaimed)))
	self.JsonSuccessResponse(http)
	return
}
