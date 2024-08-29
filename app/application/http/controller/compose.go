package controller

import (
	"encoding/json"
	"errors"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type Compose struct {
	controller.Abstract
}

func (self Compose) Create(http *gin.Context) {
	type ParamsValidate struct {
		Title       string `json:"title" binding:"required"`
		Name        string `json:"name" binding:"required"`
		Yaml        string `json:"yaml" binding:"required"`
		RawYaml     string `json:"rawYaml"`
		Environment []accessor.EnvItem
		Id          int32 `json:"id"`
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

	_, err := docker.NewYaml(params.Yaml)
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
		Name string `json:"name"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	type item struct {
		ID          int32  `json:"id"`
		Name        string `json:"name"`
		Title       string `json:"title"`
		Status      string `json:"status"`
		ConfigFiles string `json:"configFiles"`
		Yaml        string `json:"yaml"`
	}
	result := make([]*item, 0)
	out := logic.Compose{}.Ls(params.Name)
	err := json.Unmarshal([]byte(out), &result)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	composeList, _ := dao.Compose.Find()
	for _, compose := range composeList {
		for i, row := range result {
			if row.Name == compose.Name {
				result[i].ID = compose.ID
				result[i].Title = compose.Title
				result[i].Yaml = compose.Yaml
				break
			}
		}
	}
	self.JsonResponseWithoutError(http, gin.H{
		"list": result,
	})
	return
}

func (self Compose) GetDetail(http *gin.Context) {
	type ParamsValidate struct {
		Id   int32  `json:"id"`
		Name string `json:"name"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	var yamlRow *entity.Compose

	if params.Id > 0 {
		yamlRow, _ = dao.Compose.Where(dao.Compose.ID.Eq(params.Id)).First()
	}
	if params.Name != "" {
		yamlRow, _ = dao.Compose.Where(dao.Compose.Name.Eq(params.Name)).First()
	}
	self.JsonResponseWithoutError(http, gin.H{
		"detail": yamlRow,
	})
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
