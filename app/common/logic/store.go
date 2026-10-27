package logic

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	exec2 "os/exec"
	path2 "path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/compose-spec/compose-go/v2/dotenv"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/errdefs"
	"github.com/donknap/dpanel/app/common/logic/baota"
	"github.com/donknap/dpanel/app/common/logic/onepanel"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/compose"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/exec/local"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"gopkg.in/yaml.v3"
)

type StoreLogoFileSystem struct {
	fs.FS
}

func (self StoreLogoFileSystem) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, fs.ErrNotExist
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".md", ".png":
	default:
		return nil, fs.ErrNotExist
	}

	root, err := os.OpenRoot(storage.Local{}.GetStorePath())
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = root.Close()
	}()

	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fs.ErrNotExist
	}
	return root.Open(name)
}

type Store struct {
}

type StoreAppComposeOption struct {
	accessor.InstallAppDetail
	TaskName    string
	Username    string
	Environment []types.EnvItem
}

type ComposeDetail struct {
	Detail      *entity.Compose
	Yaml        [2]string
	ComposeFile string
	SourceDir   string
	TempDir     string
}

func (self Store) GetAppComposeDetail(storeRow *entity.Store, option StoreAppComposeOption) (_ *ComposeDetail, err error) {
	if storeRow.Setting == nil {
		return nil, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted)
	}
	app, _, ok := function.PluckArrayItemWalk(storeRow.Setting.Apps, func(item accessor.StoreAppItem) bool {
		return item.Name == option.AppName
	})
	if !ok {
		return nil, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted)
	}
	if !function.IsSafeString(storeRow.Name, function.StringRuleName) || !function.IsSafeString(app.Name, function.StringRuleName) {
		return nil, os.ErrPermission
	}
	version, ok := app.Version[option.Version]
	if !ok {
		return nil, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted)
	}
	for count := 0; version.Ref != ""; count++ {
		if count >= len(app.Version) {
			return nil, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted)
		}
		version, ok = app.Version[version.Ref]
		if !ok {
			return nil, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted)
		}
	}
	composePath, err := filepath.Rel(storeRow.Name, function.SlashPathToSystem(version.ComposeFile))
	if err != nil || !filepath.IsLocal(composePath) || composePath == "." {
		return nil, os.ErrPermission
	}
	detail := &ComposeDetail{
		Detail: &entity.Compose{
			Title:   app.Title,
			Setting: &accessor.ComposeSettingOption{Type: accessor.ComposeTypeStore},
		},
		ComposeFile: filepath.Base(composePath),
		SourceDir:   filepath.Join(storage.Local{}.GetStorePath(), storeRow.Name, filepath.Dir(composePath)),
	}
	defer func() {
		if err != nil && detail.TempDir != "" {
			err = errors.Join(err, os.RemoveAll(detail.TempDir))
		}
	}()
	var root *os.Root
	if storeRow.Setting.Type == define.StoreTypeBaoTa {
		detail.TempDir, err = storage.Local{}.CreateTempDir("")
		if err != nil {
			return nil, err
		}
		if err = self.SyncByZip(detail.TempDir, version.Download, option.AppName); err != nil {
			return nil, err
		}
		root, err = os.OpenRoot(detail.TempDir)
		if err != nil {
			return nil, err
		}
		composePath = filepath.Join(option.AppName, detail.ComposeFile)
		detail.SourceDir = filepath.Join(detail.TempDir, option.AppName)
	} else {
		storeRoot, err := os.OpenRoot(storage.Local{}.GetStorePath())
		if err != nil {
			return nil, err
		}
		defer func() { _ = storeRoot.Close() }()
		root, err = storeRoot.OpenRoot(storeRow.Name)
		if err != nil {
			return nil, err
		}
	}
	defer func() { _ = root.Close() }()
	appRoot, err := root.OpenRoot(filepath.Dir(composePath))
	if err != nil {
		return nil, err
	}
	defer func() { _ = appRoot.Close() }()
	for index, name := range []string{detail.ComposeFile, define.ComposeProjectDeployOverrideFileName} {
		info, err := appRoot.Stat(name)
		if index == 1 && errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, os.ErrPermission
		}
		content, err := appRoot.ReadFile(name)
		if err != nil {
			return nil, err
		}
		if index == 0 && len(content) == 0 {
			return nil, fmt.Errorf("store app %q version %q: empty compose YAML", option.AppName, option.Version)
		}
		detail.Yaml[index] = string(content)
	}

	environment := append(make([]types.EnvItem, 0, len(option.Environment)), option.Environment...)
	// 提交时以界面中的变量为准，只有读取初值时才补充商店默认值和 .env。
	if option.Environment == nil {
		environment = append(environment, version.Environment...)
		if storeRow.Setting.Type == define.StoreTypeOnePanel || storeRow.Setting.Type == define.StoreTypeOnePanelLocal {
			environment = append(environment, onepanel.DefaultEnv[option.AppName]...)
			environment = append(environment, onepanel.CommonEnv[define.StoreEnvContainerName])
		}
	}
	for index, item := range environment {
		if item.Rule != nil {
			rule := *item.Rule
			item.Rule = &rule
		}
		if option.Environment == nil && storeRow.Setting.Type == define.StoreTypeBaoTa {
			if value, _, ok := function.PluckMapItemWalk(baota.CommonEnv, func(_ int, value types.EnvItem) bool {
				return value.Name == item.Name
			}); ok {
				item.Value = value.Value
				if item.Rule == nil {
					item.Rule = &types.EnvValueRule{}
				}
				item.Rule.Kind |= value.Rule.Kind
				item.Rule.Option = value.Rule.Option
			}
		}
		environment[index] = item
	}
	detail.Detail.Setting.Environment = make([]types.EnvItem, 0, len(environment))
	envIndexMap := make(map[string]int, len(environment))
	for _, item := range environment {
		if index, ok := envIndexMap[item.Name]; ok {
			detail.Detail.Setting.Environment[index] = item
		} else {
			envIndexMap[item.Name] = len(detail.Detail.Setting.Environment)
			detail.Detail.Setting.Environment = append(detail.Detail.Setting.Environment, item)
		}
	}
	if option.Environment == nil {
		info, err := appRoot.Stat(define.ComposeDefaultEnvFileName)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if err == nil {
			if !info.Mode().IsRegular() {
				return nil, os.ErrPermission
			}
			content, err := appRoot.ReadFile(define.ComposeDefaultEnvFileName)
			if err != nil {
				return nil, err
			}
			fileEnv, err := dotenv.UnmarshalWithLookup(string(content), nil)
			if err != nil {
				return nil, fmt.Errorf("parse store app .env: %w", err)
			}
			envNameList := make([]string, 0, len(fileEnv))
			for name := range fileEnv {
				envNameList = append(envNameList, name)
			}
			sort.Strings(envNameList)
			for _, name := range envNameList {
				if index, ok := envIndexMap[name]; ok {
					detail.Detail.Setting.Environment[index].Value = fileEnv[name]
				} else {
					detail.Detail.Setting.Environment = append(detail.Detail.Setting.Environment, types.EnvItem{
						Name: name, Value: fileEnv[name],
						Rule: &types.EnvValueRule{Kind: types.EnvValueRuleInEnvFile},
					})
				}
			}
		}
	}
	valueReplaceTable := function.NewReplacerTable(compose.ValueReplaceTable...)
	valueReplaceTable = append(valueReplaceTable, func(value *string) {
		*value = function.StringReplaceAll(*value, compose.PlaceholderAppName, app.Name)
		*value = function.StringReplaceAll(*value, compose.PlaceholderAppVersion, option.Version)
		*value = function.StringReplaceAll(*value, compose.PlaceholderCurrentUsername, option.Username)
	})
	detail.Detail.Name = option.TaskName
	function.Placeholder(&detail.Detail.Name, valueReplaceTable...)
	if option.Environment != nil {
		valueReplaceTable = nil
	}
	valueReplaceTable = append(valueReplaceTable, func(value *string) {
		if detail.Detail.Name != "" {
			*value = function.StringReplaceAll(*value, compose.PlaceholderAppTaskName, detail.Detail.Name)
		}
	})
	for index, item := range detail.Detail.Setting.Environment {
		if option.Environment == nil {
			if strings.Contains(item.Value, compose.PlaceholderXkStoragePath) && item.Rule == nil {
				item.Rule = &types.EnvValueRule{}
			}
			function.Placeholder(&item, compose.EnvItemReplaceTable...)
		}
		function.Placeholder(&item.Value, valueReplaceTable...)
		detail.Detail.Setting.Environment[index] = item
	}
	return detail, nil
}

func (self Store) InstallApp(dockerSdk *docker.Client, storeType string, detail *ComposeDetail, targetPath string) ([]string, error) {
	if name, ok := map[string]string{
		define.StoreTypeOnePanel:      define.StoreTypeOnePanelNetwork,
		define.StoreTypeOnePanelLocal: define.StoreTypeOnePanelNetwork,
		define.StoreTypeBaoTa:         define.StoreTypeBaoTaNetwork,
	}[storeType]; ok {
		if _, err := dockerSdk.Client.NetworkInspect(dockerSdk.Ctx, name, network.InspectOptions{}); err != nil {
			if errdefs.IsNotFound(err) {
				_, err = dockerSdk.Client.NetworkCreate(dockerSdk.Ctx, name, network.CreateOptions{})
			}
			if err != nil {
				return nil, err
			}
		}
	}
	// 表单配置已由 Compose.Create 保存，模板只补充附属文件。
	fileList, err := function.CopyDir(targetPath, detail.SourceDir, function.CopyDirOption{
		Mode: function.CopyModeSkip,
		ExcludeFileList: []string{
			detail.ComposeFile, define.ComposeProjectDeployOverrideFileName, define.ComposeDefaultEnvFileName,
		},
	})
	if err != nil {
		return fileList, err
	}
	envLineList := make([]string, 0, len(detail.Detail.Setting.Environment))
	for _, item := range detail.Detail.Setting.Environment {
		envLineList = append(envLineList, fmt.Sprintf("%s=%s", item.Name, strconv.Quote(strings.ReplaceAll(item.Value, "$", "$$"))))
	}
	if err = os.WriteFile(filepath.Join(targetPath, define.ComposeDefaultEnvFileName), []byte(strings.Join(envLineList, "\n")), 0o600); err != nil {
		return fileList, err
	}
	if storeType == define.StoreTypeOnePanel || storeType == define.StoreTypeOnePanelLocal {
		initScript := filepath.Join(targetPath, "scripts", "init.sh")
		if _, err := os.Stat(initScript); err == nil {
			if cmd, err := local.New(
				local.WithCommandName("sh"),
				local.WithArgs(initScript),
				local.WithDir(targetPath),
			); err == nil {
				defer func() { _ = cmd.Close() }()
				if err = cmd.Run(); err != nil {
					slog.Debug("store deploy run init.sh", "error", err)
				}
			}
		}
	}
	return fileList, nil
}

type SyncByGitOption struct {
	TargetPath       string
	TempDownloadPath string
}

func (self Store) SyncByGit(gitUrl string, option SyncByGitOption) error {
	if _, err := exec2.LookPath("git"); err != nil {
		return function.ErrorMessage(define.ErrorMessageSystemStoreNotFoundGit)
	}
	var branch string

	if b, a, ok := strings.Cut(gitUrl, "#"); ok {
		gitUrl = b
		branch = a
	} else {
		branch = ""
	}

	// 先创建一个临时目录，下载完成后再同步数据，否则失败时原先的数据会被删除
	if option.TempDownloadPath == "" {
		// 仅当内部生成临时目录的时候才删除，如果是外部传递的，由外部来维护
		option.TempDownloadPath, _ = storage.Local{}.CreateTempDir("")
		defer func() {
			_ = os.RemoveAll(option.TempDownloadPath)
		}()
	}

	slog.Debug("store git download", "path", option.TempDownloadPath)

	args := []string{
		"clone", "--depth", "1",
	}
	if branch != "" {
		args = append(args, "-b", branch)
	}
	args = append(args, gitUrl, option.TempDownloadPath)
	cmd, err := local.New(
		local.WithCommandName("git"),
		local.WithArgs(args...),
		local.WithEnv(os.Environ()),
	)
	if err != nil {
		return err
	}
	time.AfterFunc(time.Minute*5, func() {
		_ = cmd.Close()
	})
	_, err = cmd.RunWithResult()
	if err != nil {
		return err
	}
	if option.TargetPath != "" {
		err = os.RemoveAll(option.TargetPath)
		if err != nil {
			return err
		}
		err = os.CopyFS(option.TargetPath, os.DirFS(option.TempDownloadPath))
		if err != nil {
			return err
		}
	}
	return nil
}

// SyncByZip 同步远程 zip
// root 只同步 root 目录下的内容
func (self Store) SyncByZip(path, zipUrl string, root string) error {
	zipTempFile, err := storage.Local{}.CreateTempFile("")
	if err != nil {
		return err
	}
	defer func() {
		_ = zipTempFile.Close()
		_ = os.RemoveAll(zipTempFile.Name())
	}()

	response, err := function.SafeHTTPGet(context.Background(), zipUrl, 30*time.Minute, 20<<30)
	if err != nil {
		return err
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode != http.StatusOK {
		return function.ErrorMessage(define.ErrorMessageSystemStoreDownloadFailed, "url", zipUrl, "error", response.Status)
	}
	_, err = io.Copy(zipTempFile, response.Body)
	if err != nil {
		return err
	}
	zipArchive, err := zip.OpenReader(zipTempFile.Name())
	if err != nil {
		return err
	}
	defer func() {
		_ = zipArchive.Close()
	}()
	for _, file := range zipArchive.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if strings.HasPrefix(file.Name, "__MACOSX") {
			continue
		}

		archiveFilePath := strings.TrimPrefix(path2.Clean("/"+file.Name), "/")
		if archiveFilePath == "" || archiveFilePath == "." {
			continue
		}
		if root != "" {
			rootPath := strings.TrimPrefix(path2.Clean("/"+root), "/")
			if archiveFilePath != rootPath && !strings.HasPrefix(archiveFilePath, rootPath+"/") {
				continue
			}
		}
		targetFilePath := function.SafePathJoin(path, archiveFilePath)
		err = func() error {
			err = os.MkdirAll(filepath.Dir(targetFilePath), os.ModePerm)
			if err != nil {
				return err
			}
			targetFile, err := os.OpenFile(targetFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
			if err != nil {
				return err
			}
			defer func() {
				_ = targetFile.Close()
			}()

			sourceFile, err := file.Open()
			if err != nil {
				return err
			}
			defer func() {
				_ = sourceFile.Close()
			}()

			_, err = io.Copy(targetFile, sourceFile)
			return err
		}()
		if err != nil {
			return err
		}
	}
	return nil
}

func (self Store) SyncByUrl(targetPath, url string) error {
	_ = os.MkdirAll(filepath.Dir(targetPath), os.ModePerm)
	file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	response, err := function.SafeHTTPGet(context.Background(), url, 30*time.Minute, 20<<30)
	if err != nil {
		return err
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusOK {
		return function.ErrorMessage(define.ErrorMessageSystemStoreDownloadFailed, "url", url, "error", response.Status)
	}
	_, err = io.Copy(file, response.Body)
	if err != nil {
		return err
	}
	return nil
}

func (self Store) GetAppByCasaos(storePath string) ([]accessor.StoreAppItem, error) {
	if !filepath.IsAbs(storePath) {
		storePath = filepath.Join(storage.Local{}.GetStorePath(), storePath, "Apps")
	}
	result := make([]accessor.StoreAppItem, 0)

	err := filepath.WalkDir(storePath, func(path string, d fs.DirEntry, err error) error {
		if path == storePath {
			return nil
		}
		if !d.IsDir() {
			return nil
		}

		appName, _ := filepath.Rel(storePath, path)
		appPath := filepath.Join(storePath, appName)

		storeItem := accessor.StoreAppItem{
			Name:     appName,
			Version:  make(map[string]accessor.StoreAppVersionItem),
			Contents: make(map[string]string),
		}

		composeYaml, err := os.ReadFile(filepath.Join(appPath, "docker-compose.yml"))
		if err != nil {
			return err
		}
		yamlData := new(function.ConfigMap)
		err = yaml.Unmarshal(composeYaml, &yamlData)
		if err != nil {
			return err
		}
		storeItem.Description = yamlData.GetString("x-casaos.description.zh_cn")
		storeItem.Descriptions = map[string]string{
			define.LangZh: yamlData.GetString("x-casaos.description.zh_cn"),
			define.LangEn: yamlData.GetString("x-casaos.description.en_us"),
		}
		storeItem.Tag = []string{
			yamlData.GetString("x-casaos.category"),
		}
		storeItem.Logo = yamlData.GetString("x-casaos.icon")
		if v := yamlData.GetString("x-casaos.tips.before_install.zh_cn"); v != "" {
			storeItem.Content = "markdown-file://" + v
			storeItem.Contents[define.LangZh] = "markdown://" + v
		}
		if v := yamlData.GetString("x-casaos.tips.before_install.en_us"); v != "" {
			storeItem.Contents[define.LangEn] = "markdown://" + v
		}
		resourcePath, _ := filepath.Rel(filepath.Dir(filepath.Dir(storePath)), appPath)
		storeItem.Version["latest"] = accessor.StoreAppVersionItem{
			Name:        "latest",
			ComposeFile: filepath.Join(resourcePath, "docker-compose.yml"),
			Environment: make([]types.EnvItem, 0),
		}
		if err == nil {
			result = append(result, storeItem)
		}
		return filepath.SkipDir
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}

func (self Store) ParseSettingField(field map[string]string, call func(item *types.EnvValueRule)) *types.EnvValueRule {
	valueRule := &types.EnvValueRule{}

	if field["required"] == "true" {
		valueRule.Kind |= types.EnvValueRuleRequired
	}
	if field["disabled"] == "true" {
		valueRule.Kind |= types.EnvValueRuleDisabled
	}

	switch field["type"] {
	case "text":
		valueRule.Kind |= types.EnvValueTypeText
		break
	case "number":
		valueRule.Kind |= types.EnvValueTypeNumber
		break
	case "select":
		if field["multiple"] == "true" {
			valueRule.Kind |= types.EnvValueTypeSelectMultiple
		} else {
			valueRule.Kind |= types.EnvValueTypeSelect
		}
	}

	if call != nil {
		call(valueRule)
	}
	return valueRule
}
