package controller

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/network"
	logic2 "github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/app/common/logic/onepanel"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/compose"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/donknap/dpanel/common/types/event"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type Store struct {
	controller.Abstract
}

func (self Store) Create(http *gin.Context) {
	type ParamsValidate struct {
		Id    int32                   `json:"id"`
		Title string                  `json:"title" binding:"required"`
		Type  string                  `json:"type" binding:"required"`
		Name  string                  `json:"name" binding:"required"`
		Url   string                  `json:"url" binding:"required"`
		Apps  []accessor.StoreAppItem `json:"apps" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	if params.Id <= 0 {
		storeRow, _ := dao.Store.Where(dao.Store.Name.Eq(params.Name)).First()
		if storeRow != nil {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonIdAlreadyExists, "name", params.Name), 500)
			return
		}
	} else {
		storeRow, _ := dao.Store.Where(dao.Store.ID.Eq(params.Id)).First()
		if storeRow == nil {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
			return
		}
	}

	storeNew := &entity.Store{
		Title: params.Title,
		Name:  params.Name,
		Setting: &accessor.StoreSettingOption{
			Type:      params.Type,
			Url:       params.Url,
			UpdatedAt: time.Now().Unix(),
			Apps:      params.Apps,
		},
	}
	var err error
	if params.Id <= 0 {
		err = dao.Store.Create(storeNew)
	} else {
		_, err = dao.Store.Where(dao.Store.ID.Eq(params.Id)).Updates(storeNew)
		storeNew.ID = params.Id
	}

	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	self.JsonSuccessResponse(http)
	return
}

func (self Store) Delete(http *gin.Context) {
	type ParamsValidate struct {
		Id []int32 `json:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	for _, id := range params.Id {
		storeRow, _ := dao.Store.Where(dao.Store.ID.Eq(id)).First()
		if storeRow == nil {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
			return
		}
		err := os.RemoveAll(filepath.Join(storage.Local{}.GetStorePath(), storeRow.Name))
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		_, err = dao.Store.Where(dao.Store.ID.Eq(id)).Delete()
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}

		facade.GetEvent().Publish(event.StoreDeleteEvent, event.StorePayload{
			Store: storeRow,
			Ctx:   http,
		})
	}

	self.JsonSuccessResponse(http)
	return
}

func (self Store) GetList(http *gin.Context) {
	type ParamsValidate struct {
		Title string `json:"title"`
		Name  string `json:"name"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	var list []*entity.Store

	query := dao.Store.Order(dao.Store.ID.Desc())
	if params.Title != "" {
		query = query.Where(dao.Store.Title.Like("%" + params.Title + "%"))
	}
	if params.Name != "" {
		query = query.Where(dao.Store.Name.Like("%" + params.Name + "%"))
	}
	list, _ = query.Find()

	// 如果是本地商店，同步一遍数据
	for _, item := range list {
		if item.Setting.Type == accessor.StoreTypeOnePanelLocal {
			if appList, err := (logic.Store{}).GetAppByOnePanel(item.Name); err == nil {
				item.Setting.Apps = appList
				_ = dao.Store.Save(item)
			}
		}
	}

	self.JsonResponseWithoutError(http, gin.H{
		"list": list,
	})
	return
}

func (self Store) Sync(http *gin.Context) {
	type ParamsValidate struct {
		Id   int32  `json:"id"`
		Name string `json:"name" binding:"required"`
		Type string `json:"type" binding:"required"`
		Url  string `json:"url" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	var err error

	storeRootPath := filepath.Join(storage.Local{}.GetStorePath(), params.Name)
	if _, err = os.Stat(storeRootPath); err != nil && params.Type == accessor.StoreTypeOnePanelLocal {
		_ = os.MkdirAll(filepath.Join(storeRootPath, "apps"), os.ModePerm)
	}

	appList := make([]accessor.StoreAppItem, 0)
	if params.Type == accessor.StoreTypeOnePanel || params.Type == accessor.StoreTypeOnePanelLocal {
		if params.Type == accessor.StoreTypeOnePanel {
			err = logic.Store{}.SyncByGit(params.Url, logic.SyncByGitOption{
				TargetPath: storeRootPath,
			})
			if err != nil {
				_ = notice.Message{}.Error(".gitPullEarlyEOF", "name", params.Name, "url", params.Url)
				self.JsonResponseWithError(http, err, 500)
				return
			}
		}
		appList, err = logic.Store{}.GetAppByOnePanel(params.Name)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	} else if params.Type == accessor.StoreTypeCasaOs {
		err = logic.Store{}.SyncByZip(storeRootPath, params.Url, "Apps")
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		appList, err = logic.Store{}.GetAppByCasaos(params.Name)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	} else if params.Type == accessor.StoreTypePortainer {
		err = logic.Store{}.SyncByJson(storeRootPath, params.Url)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	if params.Id > 0 {
		if storeRow, _ := dao.Store.Where(dao.Store.ID.Eq(params.Id)).First(); storeRow != nil {
			if len(appList) > 0 {
				storeRow.Setting.Apps = appList
			}
			storeRow.Setting.UpdatedAt = time.Now().Unix()
			_ = dao.Store.Save(storeRow)
		}
	}

	self.JsonResponseWithoutError(http, gin.H{
		"list": appList,
	})
	return
}

func (self Store) Deploy(http *gin.Context) {
	type ParamsValidate struct {
		StoreId     int32            `json:"storeId" binding:"required"`
		Name        string           `json:"name"`
		Version     string           `json:"version"`
		AppName     string           `json:"appName"`
		Title       string           `json:"title"`
		ComposeFile string           `json:"composeFile" binding:"required"`
		Environment []docker.EnvItem `json:"environment"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	runComposeList := logic2.Compose{}.Ls()
	if _, _, ok := function.PluckArrayItemWalk(runComposeList, func(item *compose.ProjectResult) bool {
		return item.Name == params.Name
	}); ok {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonIdAlreadyExists, "name", params.Name), 500)
		return
	}

	storeRow, err := dao.Store.Where(dao.Store.ID.Eq(params.StoreId)).First()
	if storeRow == nil {
		slog.Debug("sto deploy get store", "error", err)
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}

	composeYamlRealPath := filepath.Join(storage.Local{}.GetStorePath(), params.ComposeFile)

	// 适配 1panel
	if storeRow.Setting.Type == accessor.StoreTypeOnePanel || storeRow.Setting.Type == accessor.StoreTypeOnePanelLocal {
		if _, err := docker.Sdk.Client.NetworkInspect(docker.Sdk.Ctx, "1panel-network", network.InspectOptions{}); err != nil {
			if _, err = docker.Sdk.Client.NetworkCreate(docker.Sdk.Ctx, "1panel-network", network.CreateOptions{}); err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
		}
		if v, ok := onepanel.DefaultEnv[params.AppName]; ok {
			params.Environment = append(params.Environment, v...)
		}
	}

	valueReplaceTable := function.NewReplacerTable(compose.ValueReplaceTable...)
	valueReplaceTable = append(valueReplaceTable, func(v *string) {
		*v = function.StringReplaceAll(*v, compose.PlaceholderAppTaskName, params.Name)
	})
	valueReplaceTable = append(valueReplaceTable, func(v *string) {
		*v = function.StringReplaceAll(*v, compose.PlaceholderAppName, params.AppName)
	})
	valueReplaceTable = append(valueReplaceTable, func(v *string) {
		*v = function.StringReplaceAll(*v, compose.PlaceholderAppVersion, params.Version)
	})
	valueReplaceTable = append(valueReplaceTable, func(v *string) {
		if data, ok := http.Get("userInfo"); ok {
			if userInfo, ok := data.(logic.UserInfo); ok {
				*v = function.StringReplaceAll(*v, compose.PlaceholderCurrentUsername, userInfo.Username)
				return
			}
		}
	})
	function.Placeholder(&params.Name, valueReplaceTable...)

	envReplaceTable := function.NewReplacerTable(compose.EnvItemReplaceTable...)
	envReplaceTable = append(envReplaceTable, func(v *docker.EnvItem) {
		function.Placeholder(&v.Value, valueReplaceTable...)
		return
	})
	for i, item := range params.Environment {
		function.Placeholder(&item, envReplaceTable...)
		params.Environment[i] = item
	}

	composeNew := &entity.Compose{
		Name:  strings.ToLower(params.Name),
		Title: params.Title,
		Setting: &accessor.ComposeSettingOption{
			Type:        accessor.ComposeTypeStore,
			Store:       fmt.Sprintf("%s:%s@%s@%s", storeRow.Setting.Type, storeRow.Title, storeRow.Setting.Url, params.AppName),
			Environment: params.Environment,
			Uri: []string{
				filepath.Join(params.Name, filepath.Base(params.ComposeFile)),
			},
			DockerEnvName: docker.DefaultClientName,
		},
	}
	targetPath := filepath.Join(storage.Local{}.GetComposePath(""), params.Name)
	if docker.Sdk.DockerEnv.EnableComposePath {
		targetPath = filepath.Join(storage.Local{}.GetComposePath(docker.Sdk.Name), params.Name)
		composeNew.Setting.DockerEnvName = docker.Sdk.Name
	}

	err = dao.Compose.Create(composeNew)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	err = function.CopyDir(targetPath, filepath.Dir(composeYamlRealPath))
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	facade.GetEvent().Publish(event.ComposeCreateEvent, event.ComposePayload{
		Compose: composeNew,
		Ctx:     http,
	})

	self.JsonResponseWithoutError(http, gin.H{
		"id": composeNew.ID,
	})
	return
}
