package controller

import (
	"context"
	"errors"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
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
		DockerFile string `form:"dockerFile" binding:"omitempty"`
		Name       string `form:"name" binding:"required"`
		ZipFile    string `form:"zipFile" binding:"omitempty,required_without=DockerFile"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	if params.DockerFile == "" && params.ZipFile == "" {
		self.JsonResponseWithError(http, errors.New("至少需要指定Dockerfile或是Zip包"), 500)
		return
	}

	mustHasZipFile := false
	buildImageTask := &logic.BuildImageMessage{
		Name: params.Name,
	}
	addStr := []string{
		"ADD",
		"COPY",
	}
	for _, str := range addStr {
		if strings.Contains(strings.ToUpper(params.DockerFile), str) {
			mustHasZipFile = true
		}
	}

	if mustHasZipFile {
		if params.ZipFile == "" {
			self.JsonResponseWithError(http, errors.New("Dockerfile中包含添加文件操作，请上传对应的zip包"), 500)
			return
		}
	}
	if params.ZipFile != "" {
		path := os.TempDir() + "/" + params.ZipFile
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			self.JsonResponseWithError(http, errors.New("请先上传压缩包"), 500)
			return
		}
		buildImageTask.ZipPath = path
	}
	if params.DockerFile != "" {
		buildImageTask.DockerFileContent = []byte(params.DockerFile)
	}

	imageRow := &entity.Image{
		Name:       params.Name,
		Tag:        &accessor.ImageTagOption{},
		BuildGit:   "",
		Registry:   "",
		Status:     logic.STATUS_STOP,
		StatusStep: "",
		Message:    "",
		Type:       logic.IMAGE_TYPE_SELF,
	}
	dao.Image.Create(imageRow)
	buildImageTask.ImageId = imageRow.ID

	task := logic.NewDockerTask()
	task.QueueBuildImage <- buildImageTask

	self.JsonResponseWithoutError(http, gin.H{
		"imageId": imageRow.ID,
	})
	return
}

func (self Image) GetList(http *gin.Context) {
	type ParamsValidate struct {
		Page     int    `form:"page,default=1" binding:"omitempty,gt=0"`
		PageSize int    `form:"pageSize" binding:"omitempty"`
		Name     string `form:"name" binding:"omitempty"`
		Type     string `form:"type" binding:"required,oneof=all self build"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 10
	}

	if params.Type == "all" {
		go func() {
			// 同步镜像数据
			logic.ImageLogic{}.SyncImage()
		}()
	}

	query := dao.Image.Order(dao.Image.ID.Desc())

	switch params.Type {
	case "build":
		query = query.Where(dao.Image.Status.In(logic.STATUS_STOP, logic.STATUS_PROCESSING, logic.STATUS_ERROR))
		break
	case "self":
		query = query.Where(dao.Image.Status.Eq(logic.STATUS_SUCCESS)).Where(dao.Image.Type.Eq(logic.IMAGE_TYPE_SELF))
		break
	case "all":
		query = query.Where(dao.Image.Status.Eq(logic.STATUS_SUCCESS))
	}
	if params.Name != "" {
		query = query.Where(dao.Image.Name.Like("%" + params.Name + "%"))
	}
	list, total, _ := query.FindByPage((params.Page-1)*params.PageSize, params.PageSize)

	self.JsonResponseWithoutError(http, gin.H{
		"total": total,
		"page":  params.Page,
		"list":  list,
	})
	return
}

func (self Image) GetDetail(http *gin.Context) {
	type ParamsValidate struct {
		Id int32 `form:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	imageRow, _ := dao.Image.Where(dao.Image.ID.Eq(params.Id)).First()
	if imageRow == nil {
		self.JsonResponseWithError(http, errors.New("镜像不存在"), 500)
		return
	}
	sdk, err := docker.NewDockerClient()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	layer, err := sdk.Client.ImageHistory(context.Background(), imageRow.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	imageDetail, _, err := sdk.Client.ImageInspectWithRaw(context.Background(), imageRow.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"layer": layer,
		"info":  imageDetail,
	})
	return
}
