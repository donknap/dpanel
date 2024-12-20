package controller

import (
	"errors"
	"fmt"
	"github.com/docker/docker/api/types/network"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"os"
	"path/filepath"
	"strings"
	"time"
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
			self.JsonResponseWithError(http, errors.New("应用商店已经存在"), 500)
			return
		}
	} else {
		storeRow, _ := dao.Store.Where(dao.Store.ID.Eq(params.Id)).First()
		if storeRow == nil {
			self.JsonResponseWithError(http, errors.New("应用商店不存在"), 500)
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
			self.JsonResponseWithError(http, errors.New("应用商店不存在"), 500)
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
	storeRootPath := filepath.Join(storage.Local{}.GetStorePath(), params.Name)
	var err error
	appList := make([]accessor.StoreAppItem, 0)
	if params.Type == accessor.StoreTypeOnePanel {
		err = logic.Store{}.SyncByGit(storeRootPath, params.Url)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		_, err := docker.Sdk.Client.NetworkInspect(docker.Sdk.Ctx, "1panel-network", network.InspectOptions{})
		if err != nil {
			_, err = docker.Sdk.Client.NetworkCreate(docker.Sdk.Ctx, "1panel-network", network.CreateOptions{})
			if err != nil {
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

	if params.Id > 0 && len(appList) > 0 {
		if storeRow, _ := dao.Store.Where(dao.Store.ID.Eq(params.Id)).First(); storeRow != nil {
			storeRow.Setting.Apps = appList
			storeRow.Setting.UpdatedAt = time.Now().Unix()
			_, _ = dao.Store.Updates(storeRow)
		}
	}

	self.JsonResponseWithoutError(http, gin.H{
		"list": appList,
	})
	return
}

func (self Store) Deploy(http *gin.Context) {
	type ParamsValidate struct {
		StoreId     int32              `json:"storeId" binding:"required"`
		Name        string             `json:"name" binding:"required"`
		Title       string             `json:"title"`
		ComposeFile string             `json:"composeFile" binding:"required"`
		Environment []accessor.EnvItem `json:"environment"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	storeRow, _ := dao.Store.Where(dao.Store.ID.Eq(params.StoreId)).First()
	if storeRow == nil {
		self.JsonResponseWithError(http, errors.New("商店不存在"), 500)
		return
	}

	composeRow, _ := dao.Compose.Where(dao.Compose.Name.Eq(params.Name)).First()
	if composeRow != nil {
		self.JsonResponseWithError(http, errors.New("该标识已经创建过任务，请先删除，"+params.Name), 500)
		return
	}
	composeNew := &entity.Compose{
		Name:  strings.ToLower(params.Name),
		Title: params.Title,
		Yaml:  "",
		Setting: &accessor.ComposeSettingOption{
			Status:      "waiting",
			Type:        accessor.ComposeTypeStore,
			Store:       fmt.Sprintf("%s@%s", storeRow.Title, storeRow.Setting.Url),
			Environment: params.Environment,
			Uri: []string{
				filepath.Join(params.Name, filepath.Base(params.ComposeFile)),
			},
		},
	}
	err := dao.Compose.Create(composeNew)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	targetPath := filepath.Join(storage.Local{}.GetComposePath(), params.Name)
	err = os.CopyFS(targetPath, os.DirFS(filepath.Join(storage.Local{}.GetStorePath(), filepath.Dir(params.ComposeFile))))
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"id": composeNew.ID,
	})
	return
}
