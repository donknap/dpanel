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
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/compose-spec/compose-go/v2/types"
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
)

type Compose struct {
	controller.Abstract
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

	var dockerEnvName string
	var composeRow *entity.Compose

	if docker.Sdk.DockerEnv.EnableComposePath {
		dockerEnvName = docker.Sdk.DockerEnv.Name
	} else {
		dockerEnvName = define.DockerDefaultClientName
	}

	if params.Id != "" {
		composeRow, _ = logic.Compose{}.Get(params.Id)
		if composeRow == nil {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
			return
		}
		if params.Title != "" {
			composeRow.Title = params.Title
		}
		if params.Type != "" {
			composeRow.Setting.Type = params.Type
		}
	} else {
		if params.Type == accessor.ComposeTypeStoragePath {
			if !function.IsSafeName(params.Name) || params.Yaml != "" || params.YamlOverride != "" {
				self.JsonResponseWithError(http, fmt.Errorf("storage path task requires a safe name and an empty directory"), 400)
				return
			}
		}
		if params.Type == accessor.ComposeTypeStore {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageComposeDisableStore), 500)
			return
		}
		yamlExist, _ := dao.Compose.Where(dao.Compose.Name.Eq(params.Name)).Where(gen.Cond(
			datatypes.JSONQuery("setting").Equals(dockerEnvName, "dockerEnvName"),
		)...).First()
		if yamlExist != nil {
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
				filepath.Join(safeComposeName, define.ComposeProjectDeployComposeFileName),
			}
		}
	}

	if params.Yaml != "" || (params.Id == "" && composeRow.Setting.Type == accessor.ComposeTypeText) {
		err := os.MkdirAll(filepath.Dir(composeRow.Setting.GetUriFilePath()), os.ModePerm)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		err = os.WriteFile(composeRow.Setting.GetUriFilePath(), []byte(params.Yaml), 0644)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}

		overrideYamlFileName := define.ComposeProjectDeployOverrideFileName
		if composeRow.Setting.Type == accessor.ComposeTypeOutPath {
			// 外部 compose 的覆盖文件添加文件前缀，避免同目录中可能有多个文件导致重复
			overrideYamlFileName = fmt.Sprintf(define.ComposeProjectDeployOverrideOutPathFileName, composeRow.Name)
		}
		overrideYamlFilePath := filepath.Join(filepath.Dir(composeRow.Setting.GetUriFilePath()), overrideYamlFileName)
		overrideRelPath, _ := filepath.Rel(composeRow.Setting.GetWorkingDir(), overrideYamlFilePath)

		if params.YamlOverride != "" {
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
			if !function.InArray(composeRow.Setting.Uri, overrideRelPath) {
				composeRow.Setting.Uri = append(composeRow.Setting.Uri, overrideRelPath)
			}
		} else {
			if err = os.Remove(overrideYamlFilePath); err == nil {
				composeRow.Setting.Uri = slices.DeleteFunc(composeRow.Setting.Uri, func(s string) bool {
					return s == overrideRelPath
				})
			}
		}
	}

	// 清理掉值中的注释等其它信息
	newEnv, err := logic.Compose{}.ParseEnvItemValue(params.Environment)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	composeRow.Setting.Environment = newEnv

	newStoragePath := params.Id == "" && params.Type == accessor.ComposeTypeStoragePath
	if newStoragePath {
		composeRow.Setting.Dangling = true
	} else {
		tasker, warning, taskErr := (logic.Compose{}).GetTasker(composeRow)
		if errors.Is(taskErr, logic.ErrEmptyComposeFile) {
			composeRow.Setting.Dangling = true
		} else if taskErr != nil {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageComposeParseYamlIncorrect, "error", errors.Join(warning, taskErr).Error()), 500)
			return
		} else {
			if err = (logic.Compose{}).ValidateProject(tasker.Project); err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
			composeRow.Setting.Dangling = false
		}
	}
	envFilePath, _, err := composeRow.Setting.GetDefaultEnv()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	envLines := make([]string, 0, len(composeRow.Setting.Environment))
	if !newStoragePath {
		for i, item := range composeRow.Setting.Environment {
			envLines = append(envLines, fmt.Sprintf("%s=%s", item.Name, strconv.Quote(strings.ReplaceAll(item.Value, "$", "$$"))))
			if composeRow.Setting.Environment[i].Rule == nil {
				composeRow.Setting.Environment[i].Rule = &types2.EnvValueRule{Kind: types2.EnvValueRuleInEnvFile}
			} else {
				composeRow.Setting.Environment[i].Rule.Kind |= types2.EnvValueRuleInEnvFile
			}
		}
	}

	created := false
	createdDirectory := false
	if newStoragePath {
		taskDir := storage.Local{}.GetComposeProjectPath(dockerEnvName, params.Name)
		if err = os.MkdirAll(filepath.Dir(taskDir), os.ModePerm); err == nil {
			err = os.Mkdir(taskDir, os.ModePerm)
			if err == nil {
				createdDirectory = true
			} else if errors.Is(err, os.ErrExist) {
				var entries []os.DirEntry
				var info os.FileInfo
				info, err = os.Lstat(taskDir)
				if err == nil && !info.IsDir() {
					err = fmt.Errorf("compose task path is not a directory: %s", taskDir)
				}
				if err == nil {
					entries, err = os.ReadDir(taskDir)
					if err == nil && len(entries) != 0 {
						err = fmt.Errorf("compose task directory is not empty: %s", taskDir)
					}
				}
			}
		}
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	if composeRow.ID > 0 {
		composeRow.Setting.UpdatedAt = time.Now().Local().Format(time.DateTime)
		err = dao.Compose.Save(composeRow)
	} else if composeRow.Setting.Type != accessor.ComposeTypeOutPath {
		err = dao.Compose.Create(composeRow)
		created = err == nil
	}
	if err != nil {
		if createdDirectory {
			_ = os.Remove(storage.Local{}.GetComposeProjectPath(dockerEnvName, params.Name))
		}
		self.JsonResponseWithError(http, err, 500)
		return
	}
	// 新建挂载目录时不写入 .env，目录保持为空，等待用户放置 Compose 文件。
	if !newStoragePath {
		if err = os.WriteFile(envFilePath, []byte(strings.Join(envLines, "\n")), 0o600); err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	if created {
		facade.GetEvent().Publish(event.ComposeCreateEvent, event.ComposePayload{
			Compose: composeRow,
			Ctx:     http,
		})
	}

	if composeRow.Setting.Type == accessor.ComposeTypeOutPath {
		self.JsonResponseWithoutError(http, gin.H{
			"id": composeRow.Name,
		})
	} else {
		self.JsonResponseWithoutError(http, gin.H{
			"id": composeRow.ID,
		})
	}

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
	var dockerEnvName string
	if docker.Sdk.DockerEnv.EnableComposePath {
		dockerEnvName = docker.Sdk.DockerEnv.Name
	} else {
		dockerEnvName = define.DockerDefaultClientName
	}

	err := logic.Compose{}.Sync(dockerEnvName)
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
				DockerEnvName: docker.Sdk.Name,
				Environment:   make([]types2.EnvItem, 0),
				UpdatedAt:     runItem.UpdatedAt.Local().Format(time.DateTime),
			},
		}
		composeList = append(composeList, outPathTask)
	}
	for _, item := range composeList {
		_, _, configErr := (logic.Compose{}).GetTasker(item)
		item.Setting.Dangling = errors.Is(configErr, logic.ErrMissingComposeFile) || errors.Is(configErr, logic.ErrEmptyComposeFile) || errors.Is(configErr, os.ErrNotExist)
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

func (self Compose) GetTask(http *gin.Context) {
	type ParamsValidate struct {
		Id string `json:"id"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	composeRow, err := logic.Compose{}.Get(params.Id)
	if err != nil {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}

	// 查询任务中是否包含子任务
	subTask := make([]*entity.Compose, 0)

	data := gin.H{
		"yaml":          composeRow.Setting.GetYaml(),
		"task":          subTask,
		"project":       &types.Project{},
		"containerList": logic.Compose{}.Ps(composeRow.Name),
		"detail":        composeRow,
	}
	if len(composeRow.Setting.Uri) > 0 && composeRow.Setting.Uri[0] != "" {
		composeFile := composeRow.Setting.Uri[0]
		if !filepath.IsAbs(composeFile) {
			composeFile = filepath.Join(composeRow.Setting.GetWorkingDir(), composeFile)
		}
		workDir := filepath.Dir(composeFile)
		if directory, err := os.Stat(workDir); err == nil && directory.IsDir() {
			data["workingDir"] = workDir
		}
	}

	tasker, _, configErr := (logic.Compose{}).GetTasker(composeRow)
	if configErr == nil || errors.Is(configErr, logic.ErrEmptyComposeFile) {
		if tasker.Project != nil {
			data["project"] = tasker.Project
		}
		composeRow.Setting.Environment = tasker.Environment
	}

	if run := (logic.Compose{}).LsItem(composeRow.Name); run != nil {
		composeRow.Setting.Status = run.Status
		composeRow.Setting.UpdatedAt = run.UpdatedAt.Format(time.DateTime)
	}
	composeRow.Setting.Dangling = errors.Is(configErr, logic.ErrMissingComposeFile) || errors.Is(configErr, logic.ErrEmptyComposeFile) || errors.Is(configErr, os.ErrNotExist)

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
	var dockerEnvName string
	if docker.Sdk.DockerEnv.EnableComposePath {
		dockerEnvName = docker.Sdk.DockerEnv.Name
	} else {
		dockerEnvName = define.DockerDefaultClientName
	}

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
	for _, suffix := range logic.ComposeFileNameSuffix {
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
	yamlRow, err := logic.Compose{}.Get(params.Id)
	if err != nil {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}

	yaml := yamlRow.Setting.GetYaml()
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
