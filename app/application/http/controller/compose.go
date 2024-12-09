package controller

import (
	"errors"
	"fmt"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/compose"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"io"
	"log/slog"
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
		Id           int32              `json:"id"`
		Title        string             `json:"title"`
		Name         string             `json:"name" binding:"required,lowercase"`
		Type         string             `json:"type" binding:"required"`
		Yaml         string             `json:"yaml"`
		YamlOverride string             `json:"yamlOverride"`
		RemoteUrl    string             `json:"remoteUrl"`
		Environment  []accessor.EnvItem `json:"environment"`
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
		if params.Type == accessor.ComposeTypeStoragePath {
			self.JsonResponseWithError(http, errors.New("存储路径类型不能手动添加，请挂载 /dpanel/compose 目录自动发现。"), 500)
			return
		}
		if params.Type == accessor.ComposeTypeStore {
			self.JsonResponseWithError(http, errors.New("请先添加应用商店后，在商店中完成安装"), 500)
			return
		}
		yamlExist, _ := dao.Compose.Where(dao.Compose.Name.Eq(params.Name)).First()
		if yamlExist != nil {
			self.JsonResponseWithError(http, errors.New("站点标识已经存在，请更换"), 500)
			return
		}
		yamlRow = &entity.Compose{
			Title: params.Title,
			Name:  params.Name,
			Setting: &accessor.ComposeSettingOption{
				Status:      accessor.ComposeStatusWaiting,
				Type:        params.Type,
				Environment: params.Environment,
				Uri:         make([]string, 0),
				RemoteUrl:   params.RemoteUrl,
			},
		}
		if function.InArray([]string{
			accessor.ComposeTypeText, accessor.ComposeTypeRemoteUrl,
		}, params.Type) {
			yamlRow.Setting.Uri = []string{
				filepath.Join(params.Name, logic.ComposeProjectDeployFileName),
			}
		}
	}

	if params.Yaml != "" {
		err := os.MkdirAll(filepath.Dir(yamlRow.Setting.GetUriFilePath()), os.ModePerm)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		err = os.WriteFile(yamlRow.Setting.GetUriFilePath(), []byte(params.Yaml), 0644)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	if params.YamlOverride != "" {
		overrideYamlFilePath := filepath.Join(filepath.Dir(yamlRow.Setting.GetUriFilePath()), "dpanel-override.yaml")
		err := os.MkdirAll(filepath.Dir(overrideYamlFilePath), os.ModePerm)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		err = os.WriteFile(overrideYamlFilePath, []byte(params.YamlOverride), 0644)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}

		rel, _ := filepath.Rel(storage.Local{}.GetComposePath(), overrideYamlFilePath)
		if !function.InArray(yamlRow.Setting.Uri, rel) {
			yamlRow.Setting.Uri = append(yamlRow.Setting.Uri, rel)
		}
	}

	if params.Id > 0 {
		yamlRow.Title = params.Title
		yamlRow.Setting.Environment = params.Environment
		_, _ = dao.Compose.Updates(yamlRow)
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
		Name  string `json:"name"`
		Title string `json:"title"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	//同步本地目录任务
	err := logic.Compose{}.Sync()
	if err != nil {
		slog.Error("compose", "sync", err.Error())
		return
	}

	composeList := make([]*entity.Compose, 0)
	query := dao.Compose.Order(dao.Compose.ID.Desc())
	if params.Name != "" {
		query = query.Where(dao.Compose.Name.Like("%" + params.Name + "%"))
	}
	if params.Title != "" {
		query = query.Where(dao.Compose.Title.Like("%" + params.Title + "%"))
	}
	composeList, _ = query.Find()

	self.JsonResponseWithoutError(http, gin.H{
		"list": composeList,
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
		self.JsonResponseWithError(http, errors.New("任务不存在"), 500)
		return
	}
	yaml, err := yamlRow.Setting.GetYaml()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	data := gin.H{
		"detail": yamlRow,
		"yaml":   yaml,
	}
	self.JsonResponseWithoutError(http, data)
	return
}

func (self Compose) GetTask(http *gin.Context) {
	type ParamsValidate struct {
		Id int32 `json:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	yamlRow, _ := dao.Compose.Where(dao.Compose.ID.Eq(params.Id)).First()
	if yamlRow == nil {
		self.JsonResponseWithError(http, errors.New("任务不存在"), 500)
		return
	}
	tasker, err := logic.Compose{}.GetTasker(yamlRow)
	if err != nil {
		// 如果是外部任务并且获取不到yaml，则直接返回基本状态
		if yamlRow.Setting.Type == accessor.ComposeTypeOutPath {
			data := gin.H{
				"detail": yamlRow,
				"yaml":   [2]string{},
			}
			self.JsonResponseWithoutError(http, data)
			return
		}
		self.JsonResponseWithError(http, err, 500)
		return
	}
	yaml, err := tasker.GetYaml()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	data := gin.H{
		"detail":  yamlRow,
		"project": tasker.Project(),
		"yaml":    yaml,
	}

	if yamlRow.Setting.Status != accessor.ComposeStatusWaiting {
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
			if fmt.Sprintf(logic.ComposeProjectName, row.Name) == runItem.Name {
				self.JsonResponseWithError(http, errors.New("请先销毁容器"), 500)
				return
			}
		}
		_, err = dao.Compose.Where(dao.Compose.ID.Eq(id)).Delete()
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		err = os.RemoveAll(filepath.Join(storage.Local{}.GetComposePath(), row.Name))
		if err != nil {
			slog.Error("compose", "delete", err.Error())
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
		Id   int32  `json:"id"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	var composer *compose.Wrapper
	var err error

	if params.Id > 0 {
		composeRow, err := dao.Compose.Where(dao.Compose.ID.Eq(params.Id)).First()
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		tasker, err := logic.Compose{}.GetTasker(composeRow)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		composer = tasker.Composer
	} else {
		composer, err = compose.NewComposeWithYaml([]byte(params.Yaml))
	}
	if err == nil {
		self.JsonResponseWithoutError(http, gin.H{
			"project": composer.Project,
			"error":   "",
		})
	} else {
		self.JsonResponseWithoutError(http, gin.H{
			"project": nil,
			"error":   err.Error(),
		})
	}
	return
}

func (self Compose) Store(http *gin.Context) {
	storeList, err := dao.Store.Find()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"list": storeList,
	})
	return
}
