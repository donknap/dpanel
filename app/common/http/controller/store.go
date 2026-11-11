package controller

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/donknap/dpanel/app/common/logic"
	storelogic "github.com/donknap/dpanel/app/common/logic/store"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/storage"
	sourcesync "github.com/donknap/dpanel/common/service/sync"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/donknap/dpanel/common/types/event"
	"github.com/gin-gonic/gin"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"gorm.io/gorm"
)

type Store struct {
	controller.Abstract
}

func (self Store) Create(http *gin.Context) {
	type ParamsValidate struct {
		Id    int32  `json:"id" binding:"gte=0"`
		Title string `json:"title"`
		Type  string `json:"type" binding:"required"`
		Name  string `json:"name" binding:"required"`
		Url   string `json:"url" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	if !function.IsSafeString(params.Name, function.StringRuleName) {
		self.JsonResponseWithError(http, os.ErrPermission, 500)
		return
	}
	var err error
	var storeRow *entity.Store
	if params.Id == 0 {
		if row, _ := dao.Store.Where(dao.Store.Name.Eq(params.Name)).First(); row != nil {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonIdAlreadyExists, "name", params.Name), 500)
			return
		}
		storeRow = &entity.Store{
			Name:    params.Name,
			Setting: &accessor.StoreSettingOption{},
		}
	} else {
		storeRow, err = dao.Store.Where(dao.Store.ID.Eq(params.Id)).First()
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		if storeRow.Setting == nil {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
			return
		}
		if storeRow.Name != params.Name {
			self.JsonResponseWithError(http, os.ErrPermission, 500)
			return
		}
	}

	storeRow.Title = params.Title
	storeRow.Setting.Type = params.Type
	storeRow.Setting.Url = params.Url
	if params.Id == 0 {
		err = dao.Store.Create(storeRow)
	} else {
		_, err = dao.Store.Where(dao.Store.ID.Eq(params.Id)).Select(dao.Store.Title, dao.Store.Setting).Updates(storeRow)
	}

	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	self.JsonResponseWithoutError(http, gin.H{
		"id": storeRow.ID,
	})
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

	query := dao.Store.Where(
		dao.Store.Name.Neq(define.StoreSystemPluginName),
		dao.Store.Setting.IsNotNull(),
	).Order(dao.Store.ID.Desc())
	if params.Title != "" {
		query = query.Where(dao.Store.Title.Like("%" + params.Title + "%"))
	}
	if params.Name != "" {
		query = query.Where(dao.Store.Name.Like("%" + params.Name + "%"))
	}
	list, err := query.Find()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	// 如果是本地商店，同步一遍数据
	for _, item := range list {
		item.Setting.Total = len(item.Setting.Apps)
		if item.Setting.Type == define.StoreTypeOnePanelLocal {
			source, err := storelogic.New(item, filepath.Join(storage.Local{}.GetStorePath(), ".temp"))
			if err != nil {
				continue
			}
			if appList, err := source.GetList(); err == nil {
				item.Setting.Apps = appList
				_ = dao.Store.Save(item)
			}
		}
	}

	if !params.EnableAppList {
		list = function.PluckArrayWalk(list, func(item *entity.Store) (*entity.Store, bool) {
			item.Setting.Apps = make([]accessor.StoreAppDetail, 0)
			return item, true
		})
	}

	self.JsonResponseWithoutError(http, gin.H{
		"list": list,
	})
	return
}

func (self Store) CheckUpdate(http *gin.Context) {
	var params struct {
		Id int32 `json:"id" binding:"required,gt=0"`
	}
	if !self.Validate(http, &params) {
		return
	}
	storeRow, err := dao.Store.Where(dao.Store.ID.Eq(params.Id)).First()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	source, err := storelogic.New(storeRow, filepath.Join(storage.Local{}.GetStorePath(), ".temp"))
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if storeRow.Setting.Release == "" {
		self.JsonResponseWithoutError(http, gin.H{"release": "", "remoteRelease": "", "hasUpdate": true})
		return
	}
	release, err := source.Check(http.Request.Context())
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"release":       storeRow.Setting.Release,
		"remoteRelease": release,
		"hasUpdate":     storeRow.Setting.Release != release,
	})
}

func (self Store) Sync(http *gin.Context) {
	type ParamsValidate struct {
		Id int32 `json:"id" binding:"required,gt=0"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	storeRow, err := dao.Store.Where(dao.Store.ID.Eq(params.Id)).First()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if storeRow.Setting == nil {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}
	if storeRow.Setting.Url == "" {
		self.JsonResponseWithError(http, os.ErrPermission, 500)
		return
	}

	source, err := storelogic.New(storeRow, filepath.Join(storage.Local{}.GetStorePath(), ".temp"))
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	sourceList := make([]string, 0, len(storeRow.Setting.Mirror)+1)
	var daemonSourceList []string
	if storeRow.Setting.Type != define.StoreTypeOnePanelLocal {
		type sourceCheck struct {
			address  string
			duration time.Duration
		}
		checkList := make([]sourceCheck, 0, len(storeRow.Setting.Mirror))
		for _, mirror := range storeRow.Setting.Mirror {
			if mirror == storeRow.Setting.Url {
				continue
			}
			if !sourcesync.IsGit(mirror) && sourcesync.IsOci(mirror) {
				if reference, err := name.ParseReference(mirror); err == nil && reference.Context().RegistryStr() == name.DefaultRegistry {
					// Hub 来源交给 daemon，面板直连测速不能代表 daemon 加速的可用性。
					daemonSourceList = append(daemonSourceList, mirror)
					continue
				}
			}
			// 测速失败的地址仍参与同步，稳定排在测速成功的地址之后。
			checkList = append(checkList, sourceCheck{address: mirror, duration: time.Duration(1<<63 - 1)})
		}
		var waitGroup sync.WaitGroup
		for index, item := range checkList {
			waitGroup.Add(1)
			go func() {
				defer waitGroup.Done()
				address, _, _ := strings.Cut(item.address, "#")
				if !strings.HasPrefix(address, "http://") && !strings.HasPrefix(address, "https://") {
					reference, err := name.ParseReference(item.address)
					if err != nil {
						return
					}
					registry := reference.Context().Registry
					host := registry.RegistryStr()
					address = registry.Scheme() + "://" + host + "/v2/"
				}
				if duration, err := function.CheckHTTP(http.Request.Context(), address); err == nil {
					checkList[index].duration = duration
				}
			}()
		}
		waitGroup.Wait()
		sort.SliceStable(checkList, func(i, j int) bool {
			return checkList[i].duration < checkList[j].duration
		})
		for _, item := range checkList {
			sourceList = append(sourceList, item.address)
		}
	}
	slog.Info("store select url", "name", storeRow.Name, "url", sourceList)

	sourceList = append(sourceList, storeRow.Setting.Url)
	sourceList = append(sourceList, daemonSourceList...)
	var appList []accessor.StoreAppDetail
	var syncSourceURL string
	for _, address := range sourceList {
		if err = http.Request.Context().Err(); err != nil {
			break
		}
		if storeRow.Setting.Type != define.StoreTypeOnePanelLocal && !sourcesync.IsGit(address) && sourcesync.IsOci(address) {
			if reference, parseErr := name.ParseReference(address); parseErr == nil && reference.Context().RegistryStr() == name.DefaultRegistry {
				var dockerClient *docker.Client
				dockerClient, err = docker.NewClientWithUser(http)
				if err != nil {
					break
				}
				source, err = storelogic.New(storeRow, filepath.Join(storage.Local{}.GetStorePath(), ".temp"), storelogic.SourceOption{DockerClient: dockerClient})
				if err != nil {
					break
				}
			}
		}
		appList, err = source.Sync(http.Request.Context(), address)
		if err == nil {
			syncSourceURL = address
			break
		}
		if sourcesync.IsGit(address) && strings.Contains(err.Error(), "early EOF") {
			err = function.ErrorMessage(".gitPullEarlyEOF", "name", storeRow.Name, "url", address)
		}
	}

	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	storePath := function.SafePathJoin(storage.Local{}.GetStorePath(), storeRow.Name)
	release, err := sourcesync.Release(http.Request.Context(), syncSourceURL, storePath)
	if err != nil {
		slog.Warn("store sync release lookup failed", "name", storeRow.Name, "error", err)
	} else {
		storeRow.Setting.Release = release
		if err = dao.Store.Save(storeRow); err != nil {
			slog.Warn("store sync release save failed", "name", storeRow.Name, "error", err)
		}
	}
	if sourcesync.IsGit(syncSourceURL) {
		if err := os.RemoveAll(filepath.Join(storePath, ".git")); err != nil {
			slog.Warn("store sync Git directory cleanup failed", "name", storeRow.Name, "error", err)
		}
	}

	self.JsonResponseWithoutError(http, gin.H{
		"list": appList,
	})
	return
}

func (self Store) Deploy(http *gin.Context) {
	type ParamsValidate struct {
		accessor.StoreAppInstallDetail
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
		StoreAppInstallDetail: params.StoreAppInstallDetail,
		TaskName:              composeRow.Name,
		Environment:           composeRow.Setting.Environment,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
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
	params := accessor.StoreAppInstallDetail{}
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
		StoreAppInstallDetail: params,
		Username:              userInfo.Username,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
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
