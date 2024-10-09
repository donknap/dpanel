package controller

import (
	"errors"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"io"
	http2 "net/http"
	"os"
	"path/filepath"
	"strings"
)

type Compose struct {
	controller.Abstract
}

func (self Compose) Create(http *gin.Context) {
	type ParamsValidate struct {
		Id          int32  `json:"id"`
		Title       string `json:"title" binding:"required"`
		Name        string `json:"name" binding:"required"`
		Type        string `json:"type" binding:"required"`
		Yaml        string `json:"yaml"`
		RemoteUrl   string `json:"remoteUrl"`
		ServerPath  string `json:"serverPath"`
		Environment []accessor.EnvItem
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	var yamlRow *entity.Compose
	if params.Id > 0 {
		yamlRow, _ = dao.Compose.Where(dao.Compose.ID.Eq(params.Id)).First()
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

	uri := ""
	switch params.Type {
	case logic.ComposeTypeText:
		_, err := docker.NewYaml(params.Yaml)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		break
	case logic.ComposeTypeRemoteUrl:
		params.Yaml = params.RemoteUrl
		uri = params.RemoteUrl
		break
	case logic.ComposeTypeServerPath:
		params.Yaml = params.ServerPath
		uri = params.ServerPath
		break
	}

	if params.Id > 0 {
		yamlRow.Yaml = params.Yaml
		yamlRow.Title = params.Title
		yamlRow.Setting.Environment = params.Environment
		yamlRow.Setting.Type = params.Type
		yamlRow.Setting.Uri = uri
		_, _ = dao.Compose.Updates(yamlRow)
	} else {
		yamlRow = &entity.Compose{
			Title: params.Title,
			Name:  params.Name,
			Yaml:  params.Yaml,
			Setting: &accessor.ComposeSettingOption{
				Environment: params.Environment,
				Status:      "waiting",
				Type:        params.Type,
				Uri:         uri,
			},
		}
		_ = dao.Compose.Create(yamlRow)
	}

	self.JsonResponseWithoutError(http, gin.H{
		"id": yamlRow.ID,
	})
	return
}

func (self Compose) GetList(http *gin.Context) {
	type ParamsValidate struct {
		Name  string `json:"name"`
		Title string `json:"title"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	//同步本地目录任务
	logic.Compose{}.Sync()

	composeRunList := logic.Compose{}.Ls(params.Name)

	composeList := make([]*entity.Compose, 0)
	query := dao.Compose.Order(dao.Compose.ID.Desc())
	if params.Name != "" {
		query = query.Where(dao.Compose.Name.Like("%" + params.Name + "%"))
	}
	if params.Title != "" {
		query = query.Where(dao.Compose.Title.Like("%" + params.Title + "%"))
	}
	composeList, _ = query.Find()

	for _, runItem := range composeRunList {
		has := false
		for i, item := range composeList {
			if runItem.Name == item.Name {
				has = true
				composeList[i].Setting.Status = runItem.Status
			}
		}
		if params.Title == "" && !has {
			composeList = append(composeList, &entity.Compose{
				Title: "",
				Name:  runItem.Name,
				Setting: &accessor.ComposeSettingOption{
					Status: runItem.Status,
				},
			})
		}
	}
	self.JsonResponseWithoutError(http, gin.H{
		"list": composeList,
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
		params.Name = yamlRow.Name
	} else if params.Name != "" {
		yamlRow, _ = dao.Compose.Where(dao.Compose.Name.Eq(params.Name)).First()
	}
	if yamlRow == nil {
		yamlRow = &entity.Compose{
			Name:    "",
			Title:   "",
			Setting: &accessor.ComposeSettingOption{},
		}
	}

	if yamlRow.Setting.Type == logic.ComposeTypeRemoteUrl {
		response, err := http2.Get(yamlRow.Yaml)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		defer response.Body.Close()
		content, err := io.ReadAll(response.Body)
		yamlRow.Yaml = string(content)
	} else if yamlRow.Setting.Type == logic.ComposeTypeServerPath {
		content, err := os.ReadFile(yamlRow.Yaml)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		yamlRow.Yaml = string(content)
	} else if yamlRow.Setting.Type == logic.ComposeTypeStoragePath {
		content, err := os.ReadFile(filepath.Join(storage.Local{}.GetComposePath(), yamlRow.Yaml))
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		yamlRow.Yaml = string(content)
	}

	composeRunList := logic.Compose{}.Ls(params.Name)
	for _, item := range composeRunList {
		if item.Name == yamlRow.Name {
			yamlRow.Setting.Status = item.Status
			break
		}
	}

	containerList := logic.Compose{}.Ps(&logic.ComposeTaskOption{
		Name: yamlRow.Name,
		Yaml: yamlRow.Yaml,
	})

	self.JsonResponseWithoutError(http, gin.H{
		"detail":        yamlRow,
		"containerList": containerList,
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
	composeRunList := logic.Compose{}.Ls("")
	for _, id := range params.Id {
		row, err := dao.Compose.Where(dao.Compose.ID.Eq(id)).First()
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		for _, runItem := range composeRunList {
			if row.Name == runItem.Name {
				self.JsonResponseWithError(http, errors.New("请先销毁容器"), 500)
				return
			}
		}
		_, err = dao.Compose.Where(dao.Compose.ID.Eq(id)).Delete()
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Compose) GetFromUri(http *gin.Context) {
	type ParamsValidate struct {
		Uri string `json:"uri" binding:"required,uri"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	content := make([]byte, 0)
	var err error

	if strings.HasPrefix(params.Uri, "http") {
		response, err := http2.Get(params.Uri)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		defer response.Body.Close()
		content, err = io.ReadAll(response.Body)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	} else {
		content, err = os.ReadFile(params.Uri)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	self.JsonResponseWithoutError(http, gin.H{
		"content": string(content),
	})
	return
}
