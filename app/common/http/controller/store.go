package controller

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/donknap/dpanel/common/types/event"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"gorm.io/gorm"
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
		err := function.SafeDeleteAll(
			storage.Local{}.GetStorePath(),
			storeRow.Name,
		)
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
		Title         string `json:"title"`
		Name          string `json:"name"`
		EnableAppList bool   `json:"enableAppList"`
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
		item.Setting.Total = len(item.Setting.Apps)
		if item.Setting.Type == define.StoreTypeOnePanelLocal {
			storePath := function.SafePathJoin(storage.Local{}.GetStorePath(), item.Name)
			if appList, err := (logic.Store{}).GetAppByOnePanel(filepath.Join(storePath, "apps")); err == nil {
				item.Setting.Apps = appList
				_ = dao.Store.Save(item)
			}
		}
	}

	if !params.EnableAppList {
		list = function.PluckArrayWalk(list, func(item *entity.Store) (*entity.Store, bool) {
			item.Setting.Apps = make([]accessor.StoreAppItem, 0)
			return item, true
		})
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

	storeRootPath := function.SafePathJoin(storage.Local{}.GetStorePath(), params.Name)
	if _, err = os.Stat(storeRootPath); err != nil && params.Type == define.StoreTypeOnePanelLocal {
		_ = os.MkdirAll(filepath.Join(storeRootPath, "apps"), os.ModePerm)
	}

	appList := make([]accessor.StoreAppItem, 0)
	if params.Type == define.StoreTypeOnePanel || params.Type == define.StoreTypeOnePanelLocal {
		if params.Type == define.StoreTypeOnePanel {
			if filepath.Clean(storeRootPath) == filepath.Clean(storage.Local{}.GetStorePath()) {
				self.JsonResponseWithError(http, os.ErrPermission, 500)
				return
			}
			err = logic.Store{}.SyncByGit(params.Url, storeRootPath, logic.GitOption{})
			if err != nil {
				_ = notice.Message{}.Error(".gitPullEarlyEOF", "name", params.Name, "url", params.Url)
				self.JsonResponseWithError(http, err, 500)
				return
			}
		}
		appList, err = logic.Store{}.GetAppByOnePanel(filepath.Join(storeRootPath, "apps"))
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	} else if params.Type == define.StoreTypeCasaOs {
		err = logic.Store{}.SyncByZip(params.Url, storeRootPath, logic.ZipOption{
			Root: "Apps",
		})
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		appList, err = logic.Store{}.GetAppByCasaos(filepath.Join(storeRootPath, "Apps"))
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	} else if params.Type == define.StoreTypePortainer {
		jsonPath := filepath.Join(storeRootPath, "template.json")
		err = logic.Store{}.SyncByUrl(jsonPath, params.Url)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	} else if params.Type == define.StoreTypeBaoTa {
		appList, err = logic.Store{}.GetAppByBaoTa(storeRootPath, params.Url)
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
		accessor.InstallAppDetail
		Id string `json:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	id, err := strconv.ParseInt(params.Id, 10, 32)
	if err != nil || id <= 0 {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}
	dockerSdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	composeRow, err := dao.Compose.Where(dao.Compose.ID.Eq(int32(id))).First()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if composeRow.Setting == nil || composeRow.Setting.Type != accessor.ComposeTypeStore || composeRow.Setting.DockerEnvName != dockerSdk.DockerEnv.GetComposeEnvName() {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}
	storeRow, err := dao.Store.Where(dao.Store.ID.Eq(params.StoreId)).First()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	composeFile, _, err := composeRow.Setting.GetUriPath(composeRow.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	// 已保存的空变量列表也是用户输入，不再补商店默认值。
	if composeRow.Setting.Environment == nil {
		composeRow.Setting.Environment = make([]types.EnvItem, 0)
	}
	detail, err := (logic.Store{}).GetAppComposeDetail(storeRow, logic.StoreAppComposeOption{
		InstallAppDetail: params.InstallAppDetail,
		TaskName:         composeRow.Name,
		Environment:      composeRow.Setting.Environment,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		if detail.TempDir != "" {
			_ = os.RemoveAll(detail.TempDir)
		}
	}()
	envFilePath := filepath.Join(filepath.Dir(composeFile), define.ComposeDefaultEnvFileName)
	envFileInfo, err := os.Lstat(envFilePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	var envContent []byte
	if envFileInfo != nil {
		if !envFileInfo.Mode().IsRegular() {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
			return
		}
		envContent, err = os.ReadFile(envFilePath)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	storeFileList, err := (logic.Store{}).InstallApp(dockerSdk, storeRow.Setting.Type, detail, filepath.Dir(composeFile))
	defer func() {
		if err != nil {
			for index := len(storeFileList) - 1; index >= 0; index-- {
				if removeErr := os.Remove(storeFileList[index]); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
					err = errors.Join(err, removeErr)
				}
			}
			if envFileInfo == nil {
				if removeErr := os.Remove(envFilePath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
					err = errors.Join(err, removeErr)
				}
			} else {
				err = errors.Join(err, os.WriteFile(envFilePath, envContent, envFileInfo.Mode().Perm()))
			}
			self.JsonResponseWithError(http, err, 500)
		}
	}()
	if err != nil {
		return
	}
	composeRow.Setting.Environment = detail.Detail.Setting.Environment
	composeRow.Setting.Store = fmt.Sprintf("%s:%s@%s@%s", storeRow.Setting.Type, storeRow.Title, storeRow.Setting.Url, params.AppName)
	fileInfo, err := os.Stat(composeFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return
	}
	composeRow.Setting.Dangling = errors.Is(err, os.ErrNotExist) || fileInfo.Size() == 0
	composeRow.Setting.UpdatedAt = time.Now().Local().Format(time.DateTime)
	if err = dao.Compose.Save(composeRow); err != nil {
		return
	}
	self.JsonSuccessResponse(http)
}

func (self Store) GetAppDetail(http *gin.Context) {
	params := accessor.InstallAppDetail{}
	if !self.Validate(http, &params) {
		return
	}
	storeRow, err := dao.Store.Where(dao.Store.ID.Eq(params.StoreId)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted)
		}
		self.JsonResponseWithError(http, err, 500)
		return
	}
	data, _ := http.Get("userInfo")
	userInfo, _ := data.(logic.UserInfo)
	detail, err := (logic.Store{}).GetAppComposeDetail(storeRow, logic.StoreAppComposeOption{
		InstallAppDetail: params,
		Username:         userInfo.Username,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		if detail.TempDir != "" {
			_ = os.RemoveAll(detail.TempDir)
		}
	}()
	self.JsonResponseWithoutError(http, gin.H{
		"detail": gin.H{
			"id":    0,
			"name":  "",
			"title": detail.Detail.Title,
			"setting": gin.H{
				"type":        accessor.ComposeTypeStore,
				"environment": detail.Detail.Setting.Environment,
				"remoteUrl":   "",
				"dangling":    false,
			},
		},
		"yaml":          detail.Yaml,
		"error":         "",
		"project":       gin.H{},
		"containerList": []any{},
		"task":          []*entity.Compose{},
	})
}
