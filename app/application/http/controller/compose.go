package controller

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	http2 "net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/donknap/dpanel/app/application/logic"
	logic2 "github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	types2 "github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/donknap/dpanel/common/types/event"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"gorm.io/datatypes"
	"gorm.io/gen"
	"gorm.io/gen/field"
	"gorm.io/gorm"
)

type Compose struct {
	controller.Abstract
}

func (self Compose) SyncDir(http *gin.Context) {
	type ParamsValidate struct {
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
	composeRow, err := (logic.Compose{}).Get(params.Id)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	tasker, warning, err := (logic.Compose{}).GetTasker(dockerSdk, composeRow)
	if err != nil {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageComposeParseYamlIncorrect, "error", errors.Join(warning, err).Error()), 500)
		return
	}
	if err = (logic.Compose{}).ValidateRelativeBindMounts(tasker.Project); err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	sourceDir := filepath.Dir(tasker.ComposeFiles[0])
	targetDir := tasker.Project.WorkingDir
	if composeRow.Setting.Type == accessor.ComposeTypeOutPath && !composeRow.Setting.IsDelegate() {
		self.JsonSuccessResponse(http)
		return
	}
	if function.IsRunInDocker() {
		err = (logic.Compose{}).SyncLocalDir(sourceDir, tasker.WorkingDir)
	}
	if err == nil && dockerSdk.DockerEnv.IsRemote() {
		err = (logic.Compose{}).SyncRemoteDir(http, dockerSdk, sourceDir, targetDir, composeRow.Setting.Type == accessor.ComposeTypeOutPath)
	}
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
}

func (self Compose) Create(http *gin.Context) {
	type ParamsValidate struct {
		Id           string           `json:"id"`
		Title        string           `json:"title"`
		Name         string           `json:"name" binding:"required,lowercase"`
		Type         string           `json:"type" binding:"required"`
		Yaml         string           `json:"yaml"`
		YamlOverride string           `json:"yamlOverride"`
		RemoteUrl    string           `json:"remoteUrl"`
		Environment  []types2.EnvItem `json:"environment"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	dockerSdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	dockerEnvName := dockerSdk.DockerEnv.GetComposeEnvName()
	var composeRow *entity.Compose

	if params.Id != "" {
		composeRow, err = logic.Compose{}.Get(params.Id)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		if params.Title != "" {
			composeRow.Title = params.Title
		}
		if params.Type != "" {
			composeRow.Setting.Type = params.Type
		}
	} else {
		if params.Type == accessor.ComposeTypeStore {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageComposeDisableStore), 500)
			return
		}
		// 新建时应该先检测任务
		if v, _ := dao.Compose.Where(dao.Compose.Name.Eq(params.Name)).Where(gen.Cond(
			datatypes.JSONQuery("setting").Equals(dockerEnvName, "dockerEnvName"),
		)...).First(); v != nil {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonIdAlreadyExists, "name", params.Name), 500)
			return
		}
		if v := (logic.Compose{}).LsItem(params.Name); v != nil {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonIdAlreadyExists, "name", params.Name), 500)
			return
		}
		createTime := time.Now().Local().Format(time.DateTime)
		composeRow = &entity.Compose{
			Title: params.Title,
			Name:  params.Name,
			Setting: &accessor.ComposeSettingOption{
				Type:          params.Type,
				Environment:   params.Environment,
				Uri:           make([]string, 0),
				RemoteUrl:     params.RemoteUrl,
				DockerEnvName: dockerEnvName,
				CreatedAt:     createTime,
				UpdatedAt:     createTime,
			},
		}
		if function.InArray([]string{
			accessor.ComposeTypeText, accessor.ComposeTypeRemoteUrl, accessor.ComposeTypeStoragePath,
		}, params.Type) {
			safeComposeName := function.SafeFileName(params.Name)
			composeRow.Setting.Uri = []string{
				path.Join(safeComposeName, define.ComposeProjectDeployComposeFileName),
			}
		}
	}

	for index, uri := range composeRow.Setting.Uri {
		composeRow.Setting.Uri[index] = function.PathSeparatorToSlash(uri)
	}
	if composeRow.Setting.Type == accessor.ComposeTypeOutPath && len(composeRow.Setting.Uri) > 0 && composeRow.Setting.Delegate == nil {
		file, err := os.Open(function.SlashPathToSystem(composeRow.Setting.Uri[0]))
		if err == nil {
			_ = file.Close()
			composeRow.Setting.Delegate = function.Ptr(false)
		} else if errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission) {
			composeRow.Setting.Delegate = function.Ptr(true)
		} else {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	composeFile, overrideFile, err := composeRow.Setting.GetUriPath(composeRow.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	composeRow.Setting.Environment, err = (logic.Compose{}).ParseEnvItemValue(params.Environment)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	composeDir := filepath.Dir(composeFile)
	if err = os.MkdirAll(composeDir, os.ModePerm); err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	// 空输入表示未提交该文件的修改，不能清空或删除已有配置。
	if params.Yaml != "" {
		if err = os.WriteFile(composeFile, []byte(params.Yaml), 0644); err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	if params.YamlOverride != "" {
		if err = os.WriteFile(overrideFile, []byte(params.YamlOverride), 0644); err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		if composeRow.Setting.Type != accessor.ComposeTypeOutPath {
			overrideUri := path.Join(path.Dir(composeRow.Setting.Uri[0]), define.ComposeProjectDeployOverrideFileName)
			if !function.InArray(composeRow.Setting.Uri, overrideUri) {
				composeRow.Setting.Uri = append(composeRow.Setting.Uri, overrideUri)
			}
		}
	}
	if len(composeRow.Setting.Environment) > 0 {
		envLineList := make([]string, 0, len(composeRow.Setting.Environment))
		for index, item := range composeRow.Setting.Environment {
			envLineList = append(envLineList, fmt.Sprintf("%s=%s", item.Name, strconv.Quote(strings.ReplaceAll(item.Value, "$", "$$"))))
			if composeRow.Setting.Environment[index].Rule == nil {
				composeRow.Setting.Environment[index].Rule = &types2.EnvValueRule{Kind: types2.EnvValueRuleInEnvFile}
			} else {
				composeRow.Setting.Environment[index].Rule.Kind |= types2.EnvValueRuleInEnvFile
			}
		}
		envFilePath := filepath.Join(composeDir, define.ComposeDefaultEnvFileName)
		if err = os.WriteFile(envFilePath, []byte(strings.Join(envLineList, "\n")), 0o600); err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	tasker, warning, err := (logic.Compose{}).GetTasker(dockerSdk, composeRow)
	switch {
	case err == nil:
		composeRow.Setting.Environment = tasker.Environment
		composeRow.Setting.Dangling = false
	case errors.Is(err, logic.ErrMissingComposeFile), errors.Is(err, logic.ErrEmptyComposeFile):
		// 允许先保存空任务，后续再补齐配置。
		composeRow.Setting.Dangling = true
	default:
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageComposeParseYamlIncorrect, "error", errors.Join(warning, err).Error()), 500)
		return
	}
	if composeRow.Setting.Type == accessor.ComposeTypeOutPath && !composeRow.Setting.IsDelegate() {
		self.JsonResponseWithoutError(http, gin.H{
			"id": composeRow.Name,
		})
		return
	}
	createSuccess := false
	if composeRow.ID > 0 {
		composeRow.Setting.UpdatedAt = time.Now().Local().Format(time.DateTime)
		err = dao.Compose.Save(composeRow)
	} else {
		err = dao.Compose.Create(composeRow)
		createSuccess = err == nil
	}
	if err != nil {
		_ = os.Remove(composeDir)
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if createSuccess {
		facade.GetEvent().Publish(event.ComposeCreateEvent, event.ComposePayload{
			Compose: composeRow,
			Ctx:     http,
		})
	}

	self.JsonResponseWithoutError(http, gin.H{
		"id": composeRow.ID,
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
	dockerSdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	dockerEnvName := dockerSdk.DockerEnv.GetComposeEnvName()
	err = logic.Compose{}.Sync(dockerEnvName)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	composeList := make([]*entity.Compose, 0)
	query := dao.Compose.Order(dao.Compose.Name.Asc())
	keyword := ""
	if params.Name != "" {
		keyword = "%" + params.Name + "%"
		query = query.Where(field.Or(
			dao.Compose.Name.Like(keyword),
			dao.Compose.Title.Like(keyword),
		))
	}

	query = query.Where(gen.Cond(
		datatypes.JSONQuery("setting").Equals(dockerEnvName, "dockerEnvName"),
	)...)
	composeList, _ = query.Find()

	for i, item := range composeList {
		if !function.InArray([]string{
			accessor.ComposeStatusDeploying,
			accessor.ComposeStatusError,
		}, item.Setting.Status) {
			composeList[i].Setting.Status = accessor.ComposeStatusWaiting
		}
	}

	runComposeList := logic.Compose{}.Ls()
	for _, runItem := range runComposeList {
		if _, i, ok := function.PluckArrayItemWalk(composeList, func(dbItem *entity.Compose) bool {
			return dbItem.Name == runItem.Name
		}); ok {
			composeList[i].Setting.Status = runItem.Status
			composeList[i].Setting.UpdatedAt = runItem.UpdatedAt.Local().Format(time.DateTime)
			continue
		}
		if keyword != "" && !strings.Contains(runItem.Name, params.Name) {
			continue
		}
		outPathTask := &entity.Compose{
			Name:  runItem.Name,
			Title: "",
			Setting: &accessor.ComposeSettingOption{
				Status:        runItem.Status,
				Uri:           runItem.ConfigFileList,
				Type:          accessor.ComposeTypeOutPath,
				DockerEnvName: dockerEnvName,
				Environment:   make([]types2.EnvItem, 0),
				UpdatedAt:     runItem.UpdatedAt.Local().Format(time.DateTime),
			},
		}
		composeList = append(composeList, outPathTask)
	}
	for _, item := range composeList {
		_, _, err := (logic.Compose{}).GetTasker(dockerSdk, item)
		item.Setting.Dangling = errors.Is(err, logic.ErrMissingComposeFile) || errors.Is(err, logic.ErrEmptyComposeFile) || errors.Is(err, os.ErrNotExist)
	}

	sort.Slice(composeList, func(i, j int) bool {
		return composeList[i].Name < composeList[j].Name
	})

	self.JsonResponseWithoutError(http, gin.H{
		"list":          composeList,
		"containerList": logic.Compose{}.Ps(),
	})
	return
}

func (self Compose) GetDetail(http *gin.Context) {
	type ParamsValidate struct {
		Id            string `json:"id" binding:"required_without=Name"`
		Name          string `json:"name"`
		DockerEnvName string `json:"dockerEnvName" binding:"required_with=Name"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	dockerSdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if params.Id == "" {
		composeRow, err := dao.Compose.Select(dao.Compose.ID).Where(dao.Compose.Name.Eq(params.Name)).Where(gen.Cond(
			datatypes.JSONQuery("setting").Equals(params.DockerEnvName, "dockerEnvName"),
		)...).First()
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				err = function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted)
			}
			self.JsonResponseWithError(http, err, 500)
			return
		}
		params.Id = strconv.FormatInt(int64(composeRow.ID), 10)
	}
	composeRow, err := logic.Compose{}.Get(params.Id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted)
		}
		self.JsonResponseWithError(http, err, 500)
		return
	}

	// 查询任务中是否包含子任务
	subTask := make([]*entity.Compose, 0)

	tasker, warning, err := (logic.Compose{}).GetTasker(dockerSdk, composeRow)
	if err != nil {
		self.JsonResponseWithError(http, errors.Join(warning, err), 500)
		return
	}
	composeRow.Setting.Environment = tasker.Environment
	composeRow.Setting.Dangling = false
	data := gin.H{
		"yaml":          tasker.GetYaml(),
		"task":          subTask,
		"project":       tasker.Project,
		"containerList": logic.Compose{}.Ps(composeRow.Name),
		"detail":        composeRow,
		"workingDir":    tasker.Project.WorkingDir,
	}

	if run := (logic.Compose{}).LsItem(composeRow.Name); run != nil {
		composeRow.Setting.Status = run.Status
		composeRow.Setting.UpdatedAt = run.UpdatedAt.Format(time.DateTime)
	}

	self.JsonResponseWithoutError(http, data)
	return
}

func (self Compose) GetFromUri(http *gin.Context) {
	type ParamsValidate struct {
		Uri string `json:"uri" binding:"required,url"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	var err error
	content := make([]byte, 0)
	response, err := function.SafeHTTPGet(http.Request.Context(), params.Uri, 30*time.Second, 5<<20)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode != http2.StatusOK {
		self.JsonResponseWithError(http, fmt.Errorf("remote compose returned %s", response.Status), 500)
		return
	}
	content, err = io.ReadAll(response.Body)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"content": string(content),
	})
	return
}

func (self Compose) GetFromGit(http *gin.Context) {
	type ParamsValidate struct {
		Uri  string `json:"uri" binding:"required,url"`
		Name string `json:"name" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	dockerEnvName := docker.Sdk.DockerEnv.GetComposeEnvName()

	composeRow, _ := dao.Compose.Where(dao.Compose.Name.Eq(params.Name)).Where(gen.Cond(
		datatypes.JSONQuery("setting").Equals(dockerEnvName, "dockerEnvName"),
	)...).First()

	if composeRow == nil {
		createTime := time.Now().Local().Format(time.DateTime)
		composeRow = &entity.Compose{
			Name: params.Name,
			Setting: &accessor.ComposeSettingOption{
				Type:          accessor.ComposeTypeStoragePath,
				Environment:   make([]types2.EnvItem, 0),
				Uri:           []string{},
				RemoteUrl:     params.Uri,
				DockerEnvName: dockerEnvName,
				CreatedAt:     createTime,
				UpdatedAt:     createTime,
			},
		}
	}
	var err error
	targetPath := storage.Local{}.GetComposeProjectPath(dockerEnvName, params.Name)

	if strings.Contains(params.Uri, ".git") {
		err = logic2.Store{}.SyncByGit(params.Uri, logic2.SyncByGitOption{
			TargetPath: targetPath,
		})
	} else {
		response, err := function.SafeHTTPGet(http.Request.Context(), params.Uri, 30*time.Second, 5<<20)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		defer func() {
			_ = response.Body.Close()
		}()
		if response.StatusCode != http2.StatusOK {
			self.JsonResponseWithError(http, fmt.Errorf("remote compose returned %s", response.Status), 500)
			return
		}
		file, err := os.OpenFile(filepath.Join(targetPath, define.ComposeProjectDeployComposeFileName), os.O_RDWR|os.O_TRUNC, 0666)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		defer func() {
			_ = file.Close()
		}()
		_, err = io.Copy(file, response.Body)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	safeComposeName := function.SafeFileName(params.Name)
	for _, suffix := range accessor.ComposeFileNameSuffix {
		relYamlFilePath := filepath.Join(targetPath, suffix)
		if _, err = os.Stat(relYamlFilePath); err == nil {
			composeRow.Setting.Uri = append(composeRow.Setting.Uri, path.Join(safeComposeName, suffix))
			if _, err = os.Stat(filepath.Join(targetPath, define.ComposeProjectDeployOverrideFileName)); err == nil {
				composeRow.Setting.Uri = append(composeRow.Setting.Uri, path.Join(safeComposeName, define.ComposeProjectDeployOverrideFileName))
			}
			break
		}
	}
	composeRow.Setting.Uri = function.UniqueArrayWalk(composeRow.Setting.Uri, func(item string) string {
		return item
	})
	err = dao.Compose.Save(composeRow)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"id": composeRow.ID,
	})
	return
}

func (self Compose) Download(http *gin.Context) {
	type ParamsValidate struct {
		Id string `json:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	dockerSdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	yamlRow, err := logic.Compose{}.Get(params.Id)
	if err != nil {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}

	tasker, warning, err := (logic.Compose{}).GetTasker(dockerSdk, yamlRow)
	if err != nil {
		self.JsonResponseWithError(http, errors.Join(warning, err), 500)
		return
	}
	yaml := tasker.GetYaml()
	if yaml[0] == "" {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageComposeNotFoundYaml), 500)
		return
	}

	buffer := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buffer)

	if yaml[0] != "" {
		zipHeader := &zip.FileHeader{
			Name:               filepath.Join(yamlRow.Name, "compose.yaml"),
			Method:             zip.Deflate,
			UncompressedSize64: uint64(len(yaml[0])),
			Modified:           time.Now(),
		}
		writer, _ := zipWriter.CreateHeader(zipHeader)
		_, err := writer.Write([]byte(yaml[0]))
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	if yaml[1] != "" {
		zipHeader := &zip.FileHeader{
			Name:               filepath.Join(yamlRow.Name, define.ComposeProjectDeployOverrideFileName),
			Method:             zip.Deflate,
			UncompressedSize64: uint64(len(yaml[1])),
			Modified:           time.Now(),
		}
		writer, _ := zipWriter.CreateHeader(zipHeader)
		_, err := writer.Write([]byte(yaml[1]))
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	if !function.IsEmptyArray(yamlRow.Setting.Environment) {
		content := strings.Join(function.PluckArrayWalk(yamlRow.Setting.Environment, func(item types2.EnvItem) (string, bool) {
			return item.String(), true
		}), "\n")
		zipHeader := &zip.FileHeader{
			Name:               filepath.Join(yamlRow.Name, define.ComposeDefaultEnvFileName),
			Method:             zip.Deflate,
			UncompressedSize64: uint64(len(content)),
			Modified:           time.Now(),
		}
		writer, _ := zipWriter.CreateHeader(zipHeader)
		_, err := writer.Write([]byte(content))
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	_ = zipWriter.Close()

	http.Header("Content-Type", "application/zip")
	http.Header("Content-Disposition", "attachment; filename=export.zip")
	_, _ = http.Writer.Write(buffer.Bytes())
	return
}
