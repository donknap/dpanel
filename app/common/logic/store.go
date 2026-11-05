package logic

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/compose-spec/compose-go/v2/dotenv"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/errdefs"
	"github.com/donknap/dpanel/app/common/logic/store"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/compose"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/exec/local"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
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
	accessor.StoreAppInstallDetail
	TaskName    string
	Username    string
	Environment []types.EnvItem
}

type ComposeDetail struct {
	Detail      *entity.Compose
	Yaml        [2]string
	ComposeFile string
	SourceDir   string
}

func (self Store) GetAppComposeDetail(storeRow *entity.Store, option StoreAppComposeOption) (_ *ComposeDetail, err error) {
	source, err := store.New(storeRow, filepath.Join(storage.Local{}.GetStorePath(), ".temp"))
	if err != nil {
		return nil, err
	}
	if err = source.PreInstall(option.StoreAppInstallDetail); err != nil {
		return nil, err
	}
	app, version, err := store.GetApp(storeRow, option.StoreAppInstallDetail)
	if err != nil {
		return nil, err
	}
	composePath, _ := filepath.Rel(storeRow.Name, function.SlashPathToSystem(version.ComposeFile))
	detail := &ComposeDetail{
		Detail: &entity.Compose{
			Title:   app.Title,
			Setting: &accessor.ComposeSettingOption{Type: accessor.ComposeTypeStore},
		},
		ComposeFile: filepath.Base(composePath),
		SourceDir:   filepath.Join(storage.Local{}.GetStorePath(), storeRow.Name, filepath.Dir(composePath)),
	}
	storeRoot, err := os.OpenRoot(storage.Local{}.GetStorePath())
	if err != nil {
		return nil, err
	}
	defer storeRoot.Close()
	root, err := storeRoot.OpenRoot(storeRow.Name)
	if err != nil {
		return nil, err
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
	}
	for index, item := range environment {
		if item.Rule != nil {
			rule := *item.Rule
			item.Rule = &rule
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
