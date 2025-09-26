package controller

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/donknap/dpanel/app/application/logic"
	logic2 "github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/donknap/dpanel/common/types/event"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"gorm.io/datatypes"
	"gorm.io/gen"
	"io"
	http2 "net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
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
		Environment  []docker.EnvItem `json:"environment"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	dockerClient, err := logic2.Setting{}.GetDockerClient(docker.Sdk.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	var dockerEnvName string
	var composeRow *entity.Compose

	if dockerClient.EnableComposePath {
		dockerEnvName = dockerClient.Name
	} else {
		dockerEnvName = docker.DefaultClientName
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
		yamlExist, _ := dao.Compose.Where(dao.Compose.Name.Eq(params.Name)).First()
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
				filepath.Join(params.Name, define.ComposeProjectDeployFileName),
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

	// 获取任务 .env 的时候有两个数据源
	// 一个是任务目录下的 .env 文件，一个是数据库存储的环境变量
	// 优先以 .env 文件中的数据为准，编辑 yaml 时，也会将提交的 env 覆盖到 .env 文件中
	// 获取 .env 文件中的环境变量时，还需要将数据库中的规则和选项全部附加上，这样在表单中才会显示出来各种数据类型
	if envFilePath, envFileContent, err := composeRow.Setting.GetDefaultEnv(); err == nil {
		envLines := strings.Split(string(envFileContent), "\n")
		for _, item := range params.Environment {
			if _, i, ok := function.PluckArrayItemWalk(envLines, func(line string) bool {
				return strings.HasPrefix(line, item.Name+"=")
			}); ok {
				envLines[i] = fmt.Sprintf("%s=%s", item.Name, strconv.Quote(item.Value))
			} else {
				envLines = append(envLines, fmt.Sprintf("%s=%s", item.Name, strconv.Quote(item.Value)))
			}
		}
		err = os.WriteFile(envFilePath, []byte(strings.Join(envLines, "\n")), 0o600)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	composeRow.Setting.Environment = params.Environment

	// 验证 yaml 是否正确
	_, warning, err := logic.Compose{}.GetTasker(composeRow)
	if err != nil || warning != nil {
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
	dockerClient, err := logic2.Setting{}.GetDockerClient(docker.Sdk.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	if dockerClient.EnableComposePath {
		//同步本地目录任务
		err = logic.Compose{}.Sync(dockerClient.Name)
	} else {
		err = logic.Compose{}.Sync(docker.DefaultClientName)
	}

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

	dockerEnvName := dockerClient.Name
	if !dockerClient.EnableComposePath {
		dockerEnvName = docker.DefaultClientName
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
				Environment:   make([]docker.EnvItem, 0),
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

	options := logic.Compose{}.ComposeProjectOptionsFn(composeRow)
	options = append(options, cli.WithLoadOptions(func(options *loader.Options) {
		options.SkipValidation = true
	}))

	if tasker, _, err := (logic.Compose{}).GetTasker(composeRow); err == nil {
		data["project"] = tasker.Project
		// 展示的时候需要使用 tasker 中的环境变量，结合了数据库中的与 .env 文件中的
		composeRow.Setting.Environment = function.PluckMapWalkArray(tasker.Project.Environment, func(k string, v string) (docker.EnvItem, bool) {
			if dbItem, _, ok := function.PluckArrayItemWalk(composeRow.Setting.Environment, func(item docker.EnvItem) bool {
				return item.Name == k
			}); ok {
				return dbItem, true
			}
			return docker.EnvItem{
				Name:  k,
				Value: v,
				Rule: &docker.EnvValueRule{
					Kind: docker.EnvValueRuleInEnvFile,
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
			Name:               "compose.yaml",
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
			Name:               "dpanel-override.yaml",
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

	//if !function.IsEmptyArray(yamlRow.Setting.Environment) {
	//	envList := make([]string, 0)
	//	for _, item := range yamlRow.Setting.Environment {
	//		envList = append(envList, fmt.Sprintf("%s=%s", item.Name, item.Value))
	//	}
	//	content := strings.Join(envList, "\n")
	//	zipHeader := &zip.FileHeader{
	//		Name:               ".dpanel.env",
	//		Method:             zip.Deflate,
	//		UncompressedSize64: uint64(len(content)),
	//		Modified:           time.Now(),
	//	}
	//	writer, _ := zipWriter.CreateHeader(zipHeader)
	//	_, err := writer.Write([]byte(content))
	//	if err != nil {
	//		self.JsonResponseWithError(http, err, 500)
	//		return
	//	}
	//}
	_ = zipWriter.Close()

	http.Header("Content-Type", "application/zip")
	http.Header("Content-Disposition", "attachment; filename=export.zip")
	_, _ = http.Writer.Write(buffer.Bytes())
	return
}
