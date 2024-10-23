package controller

import (
	"errors"
	"fmt"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/service/compose"
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
		Id          int32                             `json:"id"`
		Title       string                            `json:"title"`
		Name        string                            `json:"name" binding:"required"`
		Type        string                            `json:"type" binding:"required"`
		Yaml        string                            `json:"yaml"`
		RemoteUrl   string                            `json:"remoteUrl"`
		ServerPath  string                            `json:"serverPath"`
		Environment []accessor.EnvItem                `json:"environment"`
		Override    map[string]accessor.SiteEnvOption `json:"override"`
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
		if params.Type == logic.ComposeTypeStoragePath {
			self.JsonResponseWithError(http, errors.New("存储路径类型不能手动添加，请挂载 /dpanel/compose 目录自动发现。"), 500)
			return
		}
		yamlExist, _ := dao.Compose.Where(dao.Compose.Name.Eq(params.Name)).First()
		if yamlExist != nil {
			self.JsonResponseWithError(http, errors.New("站点标识已经存在，请更换"), 500)
			return
		}
	}

	uri := ""
	switch params.Type {
	case logic.ComposeTypeText:
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
		yamlRow.Title = params.Title
		yamlRow.Setting.Override = params.Override
		yamlRow.Setting.Environment = params.Environment
		if params.Type != logic.ComposeTypeStoragePath {
			yamlRow.Setting.Type = params.Type
			yamlRow.Setting.Uri = uri
			yamlRow.Yaml = params.Yaml
		}
		_, _ = dao.Compose.Updates(yamlRow)
	} else {
		yamlRow = &entity.Compose{
			Title: params.Title,
			Name:  params.Name,
			Yaml:  params.Yaml,
			Setting: &accessor.ComposeSettingOption{
				Status:      logic.ComposeStatusWaiting,
				Type:        params.Type,
				Uri:         uri,
				Environment: params.Environment,
				Override:    params.Override,
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

	composeRunList := logic.Compose{}.Ls()

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
			if runItem.Name == fmt.Sprintf(logic.ComposeProjectName, item.ID) {
				has = true
				composeList[i].Setting.Status = runItem.Status
			}
		}
		if params.Title == "" && !has && strings.Contains(runItem.Name, params.Name) && !strings.Contains(runItem.Name, "dpanel-compose") {
			composeList = append(composeList, &entity.Compose{
				Title: "",
				Name:  runItem.Name,
				Setting: &accessor.ComposeSettingOption{
					Status: runItem.Status,
					Type:   logic.ComposeTypeOutPath,
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
		if yamlRow == nil {
			self.JsonResponseWithError(http, errors.New("任务不存在"), 500)
			return
		}
		params.Name = yamlRow.Name
	} else if params.Name != "" {
		yamlRow, _ = dao.Compose.Where(dao.Compose.Name.Eq(params.Name)).First()
	}
	composeRunList := logic.Compose{}.Ls()

	if yamlRow == nil {
		yamlRow = &entity.Compose{
			Name:  "",
			Title: "",
			Setting: &accessor.ComposeSettingOption{
				Type: logic.ComposeTypeServerPath,
			},
		}
		// 查不到数据表示此compose非面板管理，尝试获取名称和配置文件信息解析
		for _, item := range composeRunList {
			if item.Name == params.Name {
				yamlRow.Yaml = ""
				yamlRow.Name = item.Name
				yamlRow.Setting.Uri = item.ConfigFileList[0]
				yamlRow.Setting.Status = item.Status
				_, err := os.Stat(yamlRow.Setting.Uri)
				if err != nil {
					data := gin.H{
						"detail":  yamlRow,
						"project": nil,
					}
					self.JsonResponseWithoutError(http, data)
					return
				}
				break
			}
		}
	}
	tasker, err := logic.Compose{}.GetTasker(yamlRow)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	yaml, err := tasker.Yaml()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	yamlRow.Yaml = string(yaml)

	status := logic.ComposeStatusWaiting
	if yamlRow.Setting.Status == logic.ComposeStatusWaiting {
		for _, item := range composeRunList {
			if item.Name == tasker.Name {
				status = item.Status
				yamlRow.Setting.Status = status
				break
			}
		}
	} else {
		status = yamlRow.Setting.Status
	}

	data := gin.H{
		"detail": yamlRow,
	}
	if status != logic.ComposeStatusWaiting {
		data["containerList"] = tasker.Ps()
	}

	self.JsonResponseWithoutError(http, data)
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
	composeRunList := logic.Compose{}.Ls()
	for _, id := range params.Id {
		row, err := dao.Compose.Where(dao.Compose.ID.Eq(id)).First()
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		for _, runItem := range composeRunList {
			if fmt.Sprintf(logic.ComposeProjectName, row.ID) == runItem.Name {
				self.JsonResponseWithError(http, errors.New("请先销毁容器"), 500)
				return
			}
		}
		if row.Setting.Type != logic.ComposeTypeStoragePath {
			err = os.RemoveAll(filepath.Join(storage.Local{}.GetComposePath(), row.Name))
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
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

func (self Compose) Parse(http *gin.Context) {
	type ParamsValidate struct {
		Yaml string `json:"yaml" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	composer, err := compose.NewComposeWithYaml([]byte(params.Yaml))
	if err == nil {
		self.JsonResponseWithoutError(http, gin.H{
			"project": composer.Project,
		})
	} else {
		self.JsonResponseWithoutError(http, gin.H{
			"project": nil,
		})
	}

	return
}
