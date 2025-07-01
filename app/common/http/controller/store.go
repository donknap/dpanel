package controller

import (
	"errors"
	"fmt"
	"github.com/docker/docker/api/types/network"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/compose"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/event"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"gorm.io/datatypes"
	"gorm.io/gen"
	"log/slog"
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
			self.JsonResponseWithError(http, function.ErrorMessage(".commonIdAlreadyExists", "name", params.Name), 500)
			return
		}
	} else {
		storeRow, _ := dao.Store.Where(dao.Store.ID.Eq(params.Id)).First()
		if storeRow == nil {
			self.JsonResponseWithError(http, function.ErrorMessage(".commonDataNotFoundOrDeleted"), 500)
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
			self.JsonResponseWithError(http, function.ErrorMessage(".commonDataNotFoundOrDeleted"), 500)
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
			err = logic.Store{}.SyncByGit(storeRootPath, params.Url)
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
		AppName     string           `json:"appName"`
		Title       string           `json:"title"`
		ComposeFile string           `json:"composeFile" binding:"required"`
		Environment []docker.EnvItem `json:"environment"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	storeRow, err := dao.Store.Where(dao.Store.ID.Eq(params.StoreId)).First()
	if storeRow == nil {
		slog.Debug("sto deploy get store", "error", err)
		self.JsonResponseWithError(http, notice.Message{}.New(".commonDataNotFoundOrDeleted"), 500)
		return
	}

	if storeRow.Setting.Type == accessor.StoreTypeOnePanel {
		if _, err := docker.Sdk.Client.NetworkInspect(docker.Sdk.Ctx, "1panel-network", network.InspectOptions{}); err != nil {
			if _, err = docker.Sdk.Client.NetworkCreate(docker.Sdk.Ctx, "1panel-network", network.CreateOptions{}); err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
		}
	}

	envReplaceTable := compose.NewReplaceTable(
		func(item *docker.EnvItem) error {
			if !strings.Contains(item.Value, compose.CurrentUsername) {
				return nil
			}
			if data, ok := http.Get("userInfo"); ok {
				if userInfo, ok := data.(logic.UserInfo); ok {
					item.Value = userInfo.Username
					return nil
				}
			}
			return errors.New("not found userinfo")
		},
	)

	if strings.Contains(params.Name, compose.CurrentDate) {
		temp := docker.EnvItem{
			Value: params.Name,
		}
		if err = envReplaceTable.Replace(&temp); err == nil {
			params.Name = temp.Value
		}
	}

	params.Name = strings.ToLower(params.Name)
	total, err := dao.Compose.Where(dao.Compose.Name.Eq(params.Name)).Where(gen.Cond(datatypes.JSONQuery("setting").Equals(docker.Sdk.Name, "dockerEnvName"))...).Count()
	if total != 0 {
		self.JsonResponseWithError(http, notice.Message{}.New(".storeCreateNameExists", "name", params.Name), 500)
		return
	}

	for i, item := range params.Environment {
		if err := envReplaceTable.Replace(&item); err == nil {
			params.Environment[i] = item
		} else {
			slog.Debug("store replace env", "error", err)
		}
	}

	composeNew := &entity.Compose{
		Name:  params.Name,
		Title: params.Title,
		Setting: &accessor.ComposeSettingOption{
			Type:        accessor.ComposeTypeStore,
			Store:       fmt.Sprintf("%s@%s@%s", storeRow.Title, storeRow.Setting.Url, params.AppName),
			Environment: params.Environment,
			Uri: []string{
				filepath.Join(params.Name, filepath.Base(params.ComposeFile)),
			},
			DockerEnvName: docker.DefaultClientName,
		},
	}
	targetPath := filepath.Join(storage.Local{}.GetComposePath(), params.Name)
	if dockerClient, err := new(logic.Setting).GetDockerClient(docker.Sdk.Name); err == nil && dockerClient.EnableComposePath {
		targetPath = filepath.Join(filepath.Dir(storage.Local{}.GetComposePath()), "compose-"+dockerClient.Name, params.Name)
		composeNew.Setting.DockerEnvName = dockerClient.Name
	}

	err = dao.Compose.Create(composeNew)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	err = os.CopyFS(targetPath, os.DirFS(filepath.Join(storage.Local{}.GetStorePath(), filepath.Dir(params.ComposeFile))))
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
