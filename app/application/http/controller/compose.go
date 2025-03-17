package controller

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"github.com/compose-spec/compose-go/v2/cli"
	"github.com/donknap/dpanel/app/application/logic"
	logic2 "github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/compose"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"gorm.io/datatypes"
	"gorm.io/gen"
	"io"
	"log/slog"
	http2 "net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Compose struct {
	controller.Abstract
}

func (self Compose) Create(http *gin.Context) {
	type ParamsValidate struct {
		Id               string           `json:"id"`
		Title            string           `json:"title"`
		Name             string           `json:"name" binding:"required,lowercase"`
		Type             string           `json:"type" binding:"required"`
		Yaml             string           `json:"yaml"`
		YamlOverride     string           `json:"yamlOverride"`
		RemoteUrl        string           `json:"remoteUrl"`
		Environment      []docker.EnvItem `json:"environment"`
		DeployBackground bool             `json:"deployBackground"`
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
	var yamlRow *entity.Compose

	if dockerClient.EnableComposePath {
		dockerEnvName = dockerClient.Name
	} else {
		dockerEnvName = docker.DefaultClientName
	}

	if params.Id != "" {
		yamlRow, _ = logic.Compose{}.Get(params.Id)
		if yamlRow == nil {
			self.JsonResponseWithError(http, errors.New("站点不存在"), 500)
			return
		}
		if params.Title != "" {
			yamlRow.Title = params.Title
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
				Type:          params.Type,
				Environment:   params.Environment,
				Uri:           make([]string, 0),
				RemoteUrl:     params.RemoteUrl,
				DockerEnvName: dockerEnvName,
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
		overrideYamlFileName := "dpanel-override.yaml"
		if yamlRow.Setting.Type == accessor.ComposeTypeOutPath {
			// 外部 compose 的覆盖文件采用同名
			overrideYamlFileName = fmt.Sprintf("dpanel-%s-override.yaml", yamlRow.Name)
		}
		overrideYamlFilePath := filepath.Join(filepath.Dir(yamlRow.Setting.GetUriFilePath()), overrideYamlFileName)
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

		rel, _ := filepath.Rel(yamlRow.Setting.GetWorkingDir(), overrideYamlFilePath)
		if !function.InArray(yamlRow.Setting.Uri, rel) {
			yamlRow.Setting.Uri = append(yamlRow.Setting.Uri, rel)
		}
	}

	if !function.IsEmptyArray(params.Environment) {
		if yamlRow.Setting.Type == accessor.ComposeTypeOutPath {
			// 如果是外部任务，不插件数据库，有变动后直接修改原文件
			// 提交写入 .dpanel.env 文件
			globalEnv := function.PluckArrayWalk(params.Environment, func(i docker.EnvItem) (string, bool) {
				return fmt.Sprintf("%s=%s", i.Name, i.Value), true
			})
			envFileName := filepath.Join(filepath.Dir(yamlRow.Setting.Uri[0]), logic.ComposeDefaultEnvFileName)
			_ = os.MkdirAll(filepath.Dir(envFileName), os.ModePerm)
			err = os.WriteFile(envFileName, []byte(strings.Join(globalEnv, "\n")), 0666)
		} else {
			yamlRow.Setting.Environment = params.Environment
		}
	}

	if params.DeployBackground {
		yamlRow.Setting.Status = accessor.ComposeStatusDeploying
	} else {
		yamlRow.Setting.Status = ""
	}

	if yamlRow.ID > 0 {
		_, _ = dao.Compose.Updates(yamlRow)
	} else if yamlRow.Setting.Type != accessor.ComposeTypeOutPath {
		_ = dao.Compose.Create(yamlRow)
	}

	if yamlRow.Setting.Type == accessor.ComposeTypeOutPath {
		self.JsonResponseWithoutError(http, gin.H{
			"id": yamlRow.Name,
		})
	} else {
		self.JsonResponseWithoutError(http, gin.H{
			"id": yamlRow.ID,
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

	runComposeList := logic.Compose{}.FindRunTask()

	for i, item := range composeList {
		if !function.InArray([]string{
			accessor.ComposeStatusDeploying,
			accessor.ComposeStatusError,
		}, item.Setting.Status) {
			composeList[i].Setting.Status = accessor.ComposeStatusWaiting
		}

		if find, ok := runComposeList[item.Name]; ok {
			composeList[i].Setting.Status = find.Setting.Status
			delete(runComposeList, item.Name)
		}
	}

	for _, item := range runComposeList {
		composeList = append(composeList, item)
	}

	sort.Slice(composeList, func(i, j int) bool {
		return composeList[i].Name < composeList[j].Name
	})

	self.JsonResponseWithoutError(http, gin.H{
		"list": composeList,
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
	yamlRow, err := logic.Compose{}.Get(params.Id)
	if err != nil {
		self.JsonResponseWithError(http, errors.New("任务不存在"), 500)
		return
	}
	tasker, err := logic.Compose{}.GetTasker(yamlRow)
	if err != nil {
		// 如果获取任务失败，可能是没有文件或是Yaml文件错误，直接返回内容待用户修改
		yaml, err := yamlRow.Setting.GetYaml()
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		data := gin.H{
			"detail":        yamlRow,
			"yaml":          yaml,
			"containerList": logic.Compose{}.FilterContainer(yamlRow.Name),
		}
		self.JsonResponseWithoutError(http, data)
		return
	}

	yamlRow.Setting.Status = tasker.Status

	yaml, err := tasker.GetYaml()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	data := gin.H{
		"detail":        yamlRow,
		"project":       tasker.Project(),
		"yaml":          yaml,
		"containerList": make([]interface{}, 0),
	}

	if yamlRow.Setting.Status != accessor.ComposeStatusWaiting {
		if list := tasker.Ps(); list != nil {
			data["containerList"] = list
		}
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
		err = os.RemoveAll(filepath.Join(row.Setting.GetWorkingDir(), row.Name))
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
		defer func() {
			_ = response.Body.Close()
		}()
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
		Yaml        []string         `json:"yaml" binding:"required"`
		Id          string           `json:"id"`
		Environment []docker.EnvItem `json:"environment"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	var composer *compose.Wrapper
	var err error

	if params.Id != "" {
		composeRow, err := logic.Compose{}.Get(params.Id)
		if err != nil {
			self.JsonResponseWithError(http, errors.New("任务不存在"), 500)
			return
		}
		tasker, err := logic.Compose{}.GetTasker(composeRow)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		composer = tasker.Composer
	} else {
		options := make([]cli.ProjectOptionsFn, 0)
		if !function.IsEmptyArray(params.Environment) {
			options = append(options, compose.WithDockerEnvItem(params.Environment...))
		}
		options = append(options, compose.WithYamlContent(params.Yaml...))

		composer, err = compose.NewCompose(options...)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		_ = os.RemoveAll(composer.Project.WorkingDir)
	}

	self.JsonResponseWithoutError(http, gin.H{
		"project":     composer.Project,
		"environment": composer.Project.Environment,
		"error":       "",
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
		self.JsonResponseWithError(http, errors.New("任务不存在"), 500)
		return
	}

	yaml, err := yamlRow.Setting.GetYaml()
	if err != nil || yaml[0] == "" {
		self.JsonResponseWithError(http, notice.Message{}.New(".composeNotFoundYaml"), 500)
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
