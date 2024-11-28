package controller

import (
	"errors"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/dao"
	"github.com/gin-gonic/gin"
)

func (self Image) GetBuildTask(http *gin.Context) {
	type ParamsValidate struct {
		Id int32 `form:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	imageRow, _ := dao.Image.Where(dao.Image.ID.Eq(params.Id)).First()
	if imageRow == nil {
		self.JsonResponseWithError(http, errors.New("构建任务不存在"), 500)
		return
	}
	tagDetail := logic.Image{}.GetImageTagDetail(imageRow.Tag)
	imageRow.Tag = tagDetail.ImageName

	self.JsonResponseWithoutError(http, gin.H{
		"task": imageRow,
	})
	return
}

func (self Image) DeleteBuildTask(http *gin.Context) {
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

func (self Image) GetListBuild(http *gin.Context) {
	type ParamsValidate struct {
		Page     int  `json:"page,default=1" binding:"omitempty,gt=0"`
		PageSize int  `json:"pageSize" binding:"omitempty,gt=1"`
		All      bool `json:"all"`
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

	query := dao.Image.Order(dao.Image.ID.Desc())
	if !params.All {
		query = query.Where(dao.Image.BuildType.Neq("pull"))
	}
	list, total, _ := query.FindByPage((params.Page-1)*params.PageSize, params.PageSize)
	self.JsonResponseWithoutError(http, gin.H{
		"total": total,
		"page":  params.Page,
		"list":  list,
	})
	return
}
