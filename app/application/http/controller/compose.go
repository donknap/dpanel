package controller

import (
	"errors"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
)

type Compose struct {
	controller.Abstract
}

func (self Compose) Create(http *gin.Context) {
	type ParamsValidate struct {
		Title string `json:"title" binding:"required"`
		Name  string `json:"name" binding:"required"`
		Yaml  string `json:"yaml" binding:"required"`
		Id    int32  `json:"id"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	if params.Id > 0 {
		yamlRow, _ := dao.Compose.Where(dao.Compose.ID.Eq(params.Id)).First()
		if yamlRow == nil {
			self.JsonResponseWithError(http, errors.New("站点不存在"), 500)
			return
		}
	} else {
		yamlExist, _ := dao.Compose.Where(dao.Compose.Name.Eq(params.Name)).First()
		if yamlExist != nil {
			self.JsonResponseWithError(http, errors.New("站点标识已经存在，请更换"), 500)
			return
		}
	}

	_, err := logic.Compose{}.GetYaml(params.Yaml)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	yamlRow := &entity.Compose{
		Title: params.Title,
		Yaml:  params.Yaml,
		Name:  params.Name,
	}
	if params.Id > 0 {
		_, _ = dao.Compose.Where(dao.Compose.ID.Eq(params.Id)).Updates(yamlRow)
	} else {
		_ = dao.Compose.Create(yamlRow)
	}
	self.JsonResponseWithoutError(http, gin.H{
		"id": yamlRow.ID,
	})
	return
}

func (self Compose) GetList(http *gin.Context) {
	type ParamsValidate struct {
		Page     int    `json:"page,default=1" binding:"omitempty,gt=0"`
		PageSize int    `json:"pageSize" binding:"omitempty"`
		Title    string `json:"title"`
		Name     string `json:"name"`
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

	query := dao.Compose.Order(dao.Compose.ID.Desc())
	if params.Title != "" {
		query = query.Where(dao.Compose.Title.Like("%" + params.Title + "%"))
	}
	if params.Name != "" {
		query = query.Where(dao.Compose.Name.Like("%" + params.Name + "%"))
	}
	list, total, _ := query.FindByPage((params.Page-1)*params.PageSize, params.PageSize)

	self.JsonResponseWithoutError(http, gin.H{
		"total": total,
		"page":  params.Page,
		"list":  list,
	})
	return
}

func (self Compose) GetDetail(http *gin.Context) {
	type ParamsValidate struct {
		Id int32 `json:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	yamlRow, _ := dao.Compose.Where(dao.Compose.ID.Eq(params.Id)).First()
	if yamlRow == nil {
		self.JsonResponseWithError(http, errors.New("站点标识已经存在，请更换"), 500)
		return
	}
	logic.Compose{}.Ls(yamlRow.Name)
	self.JsonResponseWithoutError(http, yamlRow)
	return
}

func (self Compose) Delete(http *gin.Context) {
	type ParamsValidate struct {
		Id []int32 `form:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	_, err := dao.Compose.Where(dao.Compose.ID.In(params.Id...)).Delete()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}
