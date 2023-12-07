package controller

import (
	"errors"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
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
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	buildImageTask := &logic.BuildImageMessage{
		Name: params.Name,
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

	if mustHasZipFile {
		fileUploader, fileHeader, _ := http.Request.FormFile("zipFile")
		if fileHeader == nil {
			self.JsonResponseWithError(http, errors.New("Dockerfile中包含添加文件操作，请上传对应的zip包"), 500)
			return
		}
		defer fileUploader.Close()

		file, _ := os.CreateTemp("", "dpanel")
		err := http.SaveUploadedFile(fileHeader, file.Name())
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		buildImageTask.ZipPath = file.Name()
	}
	if params.DockerFile != "" {
		buildImageTask.DockerFileContent = []byte(params.DockerFile)
	}

	if buildImageTask.ZipPath == "" && buildImageTask.DockerFileContent == nil {
		self.JsonResponseWithError(http, errors.New("Dockerfile 和 Zip 至少要指定一项"), 500)
		return
	}

	imageRow := &entity.Image{
		Name:       params.Name,
		Tag:        &accessor.ImageTagOption{},
		BuildGit:   "",
		Registry:   "",
		Status:     logic.STATUS_STOP,
		StatusStep: "",
		Message:    "",
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

	go func() {
		// 同步镜像数据
		logic.ImageLogic{}.SyncImage()
	}()

	query := dao.Image.Order(dao.Image.ID.Desc())
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
