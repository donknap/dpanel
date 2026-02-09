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
	} else {
		if params.Type == accessor.ComposeTypeStoragePath {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageComposeDisableStorageType), 500)
			return
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
			accessor.ComposeTypeText, accessor.ComposeTypeRemoteUrl,
		}, params.Type) {
			composeRow.Setting.Uri = []string{
				filepath.Join(params.Name, define.ComposeProjectDeployComposeFileName),
			}
		}
	}

	if params.Yaml != "" {
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

	// 将存在于 .env 的变量的值重新写入到 .env 文件，并添加 EnvInFile 标识
	// 数据库中的环境变量包含了文件+数据库+自定义字段选项等完整数据
	if envFilePath, envFileContent, err := composeRow.Setting.GetDefaultEnv(); err == nil {
		envLines := strings.Split(string(envFileContent), "\n")
		for j, item := range composeRow.Setting.Environment {
			if _, i, ok := function.PluckArrayItemWalk(envLines, func(line string) bool {
				return strings.HasPrefix(line, item.Name+"=")
			}); ok {
				envLines[i] = fmt.Sprintf("%s=%s", item.Name, strconv.Quote(item.Value))
				if composeRow.Setting.Environment[j].Rule == nil {
					composeRow.Setting.Environment[j].Rule = &types2.EnvValueRule{
						Kind: types2.EnvValueRuleInEnvFile,
					}
				} else {
					composeRow.Setting.Environment[j].Rule.Kind |= types2.EnvValueRuleInEnvFile
				}
			}
		}
		err = os.WriteFile(envFilePath, []byte(strings.Join(envLines, "\n")), 0o600)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	// 验证 yaml 是否正确
	_, warning, err := logic.Compose{}.GetTasker(composeRow)
	if err != nil {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageComposeParseYamlIncorrect, "error", errors.Join(warning, err).Error()), 500)
		return
	}

	if composeRow.ID > 0 {
		composeRow.Setting.UpdatedAt = time.Now().Local().Format(time.DateTime)
		_ = dao.Compose.Save(composeRow)
	} else if composeRow.Setting.Type != accessor.ComposeTypeOutPath {
		_ = dao.Compose.Create(composeRow)
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
		Name  string `json:"name"`
		Title string `json:"title"`
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
	if params.Name != "" {
		query = query.Where(dao.Compose.Name.Like("%" + params.Name + "%"))
	}
	if params.Title != "" {
		query = query.Where(dao.Compose.Title.Like("%" + params.Title + "%"))
	}

	query.Where(gen.Cond(
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
		if runItem.CanManage {
			outPathTask.Setting.Type = accessor.ComposeTypeOutPath
		} else {
			outPathTask.Setting.Type = accessor.ComposeTypeDangling
		}
		composeList = append(composeList, outPathTask)
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

	if tasker, _, err := (logic.Compose{}).GetTasker(composeRow); err == nil {
		data["project"] = tasker.Project
		// 获取环境变量时，需要从 .env 文件中拿到最新值和新增的变量
		composeRow.Setting.Environment = function.PluckMapWalkArray(tasker.Project.Environment, func(k string, v string) (types2.EnvItem, bool) {
			if dbItem, _, ok := function.PluckArrayItemWalk(composeRow.Setting.Environment, func(item types2.EnvItem) bool {
				return item.Name == k
			}); ok {
				dbItem.Value = v
				return dbItem, true
			}
			return types2.EnvItem{
				Name:  k,
				Value: v,
				Rule: &types2.EnvValueRule{
					Kind: types2.EnvValueRuleInEnvFile,
				},
			}, true
		})
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
	response, err := http2.Get(params.Uri)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		_ = response.Body.Close()
	}()
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
		Uri   string `json:"uri" binding:"required,url"`
		Name  string `json:"name" binding:"required"`
		Title string `json:"title"`
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
			Title: params.Title,
			Name:  params.Name,
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
	targetPath := filepath.Join(storage.Local{}.GetComposePath(dockerEnvName), params.Name)

	if strings.Contains(params.Uri, ".git") {
		err = logic2.Store{}.SyncByGit(params.Uri, logic2.SyncByGitOption{
			TargetPath: targetPath,
		})
	} else {
		response, err := http2.Get(params.Uri)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		defer func() {
			_ = response.Body.Close()
		}()
		file, err := os.OpenFile(filepath.Join(targetPath, define.ComposeProjectDeployComposeFileName), os.O_RDWR|os.O_TRUNC, 0666)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
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

	for _, suffix := range logic.ComposeFileNameSuffix {
		relYamlFilePath := filepath.Join(targetPath, suffix)
		if _, err = os.Stat(relYamlFilePath); err == nil {
			composeRow.Setting.Uri = append(composeRow.Setting.Uri, path.Join(params.Name, suffix))
			if _, err = os.Stat(filepath.Join(targetPath, define.ComposeProjectDeployOverrideFileName)); err == nil {
				composeRow.Setting.Uri = append(composeRow.Setting.Uri, path.Join(params.Name, define.ComposeProjectDeployOverrideFileName))
			}
			break
		}
	}
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
