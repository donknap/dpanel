package controller

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/docker/docker/api/types/build"
	"github.com/docker/go-units"
	"github.com/donknap/dpanel/app/application/logic/task"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/types"
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
	if params.BuildDockerfileContent == "" && params.BuildZip == "" && params.BuildGit == "" && params.BuildPath == "" {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageImageBuildTypeEmpty), 500)
		return
	}
	if params.BuildZip != "" && params.BuildGit != "" {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageImageBuildTypeConflict), 500)
		return
	}

	if params.BuildZip != "" {
		path := storage.Local{}.GetSaveRealPath(params.BuildZip)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonUploadFileEmpty), 500)
			return
		}
		params.BuildZip = path
	}

	if params.BuildPath != "" {
		if _, err := os.Stat(params.BuildPath); err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	params.Tags = function.PluckArrayWalk(params.Tags, func(item accessor.ImageSettingTag) (accessor.ImageSettingTag, bool) {
		item.Tag = function.ImageTag(fmt.Sprintf("%s/%s", item.Registry, item.Name))
		return item, true
	})

	params.BuildSecret = function.PluckArrayWalk(params.BuildSecret, func(item types.EnvItem) (types.EnvItem, bool) {
		if v, err := function.RSAEncode(item.Value); err == nil {
			item.Value = v
		}
		return item, true
	})

	imageNew := &entity.Image{
		Tag:       "",
		BuildType: "",
		Title:     params.Title,
		Setting:   &params.ImageSettingOption,
		Status:    define.DockerImageBuildStatusStop,
		Message:   "",
	}
	imageRow, err := dao.Image.Where(dao.Image.ID.Eq(params.Id)).First()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if imageRow != nil {
		imageNew.ID = imageRow.ID
		imageNew.Status = imageRow.Status
		imageNew.Message = imageRow.Message
	}
	if err := dao.Image.Save(imageNew); err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	self.JsonResponseWithoutError(http, gin.H{
		"id": imageNew.ID,
	})
	return
}

func (self ImageBuild) Build(http *gin.Context) {
	type ParamsValidate struct {
		Id int32 `json:"id" binding:"required,gt=0"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	imageRow, err := dao.Image.Where(dao.Image.ID.Eq(params.Id)).First()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if imageRow == nil || imageRow.Setting == nil {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}
	sdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	progress, owner, err := ws.NewFdProgressPip(http, sdk.Name, fmt.Sprintf(ws.MessageTypeImageBuild, params.Id))
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if !owner {
		self.JsonResponseWithError(http, errors.New("image build is already running"), 409)
		return
	}
	defer progress.Close()
	stopWatchRequest := context.AfterFunc(http.Request.Context(), progress.Close)
	defer stopWatchRequest()
	if err := progress.Context().Err(); err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if function.IsEmptyArray(imageRow.Setting.Tags) {
		tag := imageRow.Setting.Tag
		if tag == "" {
			tag = imageRow.Tag
		}
		if tag != "" {
			imageRow.Setting.Tags = []accessor.ImageSettingTag{{
				Tag:    function.ImageTag(tag),
				Enable: true,
			}}
		}
	}
	if imageRow.Setting.BuildDockerfileContent == "" {
		imageRow.Setting.BuildDockerfileContent = imageRow.Setting.BuildDockerfile
	}
	if imageRow.Setting.BuildDockerfileRoot == "" {
		imageRow.Setting.BuildDockerfileRoot = imageRow.Setting.BuildRoot
	}
	imageRow.Status = define.DockerImageBuildStatusProcess
	imageRow.Message = ""
	if err := dao.Image.Save(imageRow); err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	startTime := time.Now()
	log, imageID, err := (task.Docker{}).Build(sdk, progress, *imageRow.Setting)
	if ctxErr := progress.Context().Err(); ctxErr != nil {
		err = ctxErr
		imageRow.Status = define.DockerImageBuildStatusStop
	} else if err != nil {
		imageRow.Status = define.DockerImageBuildStatusError
	} else {
		imageRow.Status = define.DockerImageBuildStatusSuccess
	}
	imageRow.Setting.ImageId = imageID
	imageRow.Setting.UseTime = time.Since(startTime).Seconds()
	imageRow.Message = log
	if saveErr := dao.Image.Save(imageRow); saveErr != nil {
		err = errors.Join(err, saveErr)
	}
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{"id": imageRow.ID})
}

func (self ImageBuild) GetDetail(http *gin.Context) {
	type ParamsValidate struct {
		Id int32 `json:"id" binding:"required"`
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
	if function.IsEmptyArray(imageRow.Setting.Tags) {
		tag := imageRow.Setting.Tag
		if tag == "" {
			tag = imageRow.Tag
		}
		tagDetail := function.ImageTag(tag)
		imageRow.Setting.Tag = tagDetail.Name
		imageRow.Setting.Tags = []accessor.ImageSettingTag{
			{
				Enable: true,
				Tag:    tagDetail,
			},
		}
	}

	imageRow.Setting.BuildSecret = function.PluckArrayWalk(imageRow.Setting.BuildSecret, func(item types.EnvItem) (types.EnvItem, bool) {
		if v, err := function.RSADecode(item.Value, nil); err == nil {
			item.Value = v
		}
		return item, true
	})

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
		Id []int32 `json:"id" binding:"required"`
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
		if function.IsEmptyArray(item.Setting.Tags) {

			item.Setting.Tags = []accessor.ImageSettingTag{
				{
					Tag:    function.ImageTag(item.Setting.Tag),
					Enable: true,
				},
			}
		}
		return item, true
	})
	self.JsonResponseWithoutError(http, gin.H{
		"list": list,
	})
	return
}

func (self ImageBuild) Prune(http *gin.Context) {
	sdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	res, err := sdk.Client.BuildCachePrune(sdk.Ctx, build.CachePruneOptions{
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
