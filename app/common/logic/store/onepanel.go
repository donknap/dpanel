package store

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/compose"
	"github.com/donknap/dpanel/common/service/docker/types"
	sourcesync "github.com/donknap/dpanel/common/service/sync"
	"github.com/donknap/dpanel/common/types/define"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"gopkg.in/yaml.v3"
)

type OnePanel struct{ source }

func (self OnePanel) Sync(ctx context.Context, sourceURL string) ([]accessor.StoreAppDetail, error) {
	if self.row.Setting.Type == define.StoreTypeOnePanelLocal {
		if err := os.MkdirAll(filepath.Join(self.path, "apps"), os.ModePerm); err != nil {
			return nil, err
		}
		appList, err := self.GetList()
		if err != nil {
			return nil, err
		}
		if err = self.saveList(appList); err != nil {
			return nil, err
		}
		return appList, nil
	}
	if err := os.MkdirAll(filepath.Dir(self.path), 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(self.tempPath, 0700); err != nil {
		return nil, err
	}
	stageRoot, err := os.MkdirTemp(self.tempPath, ".sync-*")
	if err != nil {
		return nil, err
	}
	defer func() {
		if stageRoot != "" {
			if err := os.RemoveAll(stageRoot); err != nil {
				slog.Warn("remove store sync temporary directory", "error", err)
			}
		}
	}()
	stagePath := filepath.Join(stageRoot, self.row.Name)
	if sourcesync.IsGit(sourceURL) {
		err = sourcesync.ByGit(ctx, sourceURL, stagePath, stageRoot, sourcesync.GitOption{})
	} else if sourcesync.IsOci(sourceURL) {
		err = sourcesync.ByOci(ctx, sourceURL, stagePath, stageRoot, sourcesync.OciOption{
			Platform:     v1.Platform{OS: "linux", Architecture: "amd64"},
			DockerClient: self.dockerClient,
		})
	} else {
		return nil, fmt.Errorf("unsupported store source address")
	}
	if err != nil {
		return nil, err
	}
	candidate := self
	candidate.path = stagePath
	appList, err := candidate.GetList()
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	storeSourceMutex.Lock()
	defer storeSourceMutex.Unlock()
	backupPath := filepath.Join(stageRoot, "previous")
	if err = os.Rename(self.path, backupPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err = os.Rename(stagePath, self.path); err != nil {
		if restoreErr := os.Rename(backupPath, self.path); restoreErr != nil && !errors.Is(restoreErr, os.ErrNotExist) {
			err = errors.Join(err, fmt.Errorf("store recovery directory %s: %w", stageRoot, restoreErr))
			stageRoot = ""
		}
		return nil, err
	}
	if err = self.saveList(appList); err != nil {
		return nil, err
	}
	return appList, nil
}

func (self OnePanel) GetList() ([]accessor.StoreAppDetail, error) {
	storePath := filepath.Join(self.path, "apps")
	result := make([]accessor.StoreAppDetail, 0)

	err := filepath.WalkDir(storePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == storePath {
			return nil
		}
		if !d.IsDir() {
			return nil
		}

		appName, _ := filepath.Rel(storePath, path)
		appPath := filepath.Join(storePath, appName)

		// 忽略不支持的应用
		ignoreApp := []string{
			"php5", "php7", "php8",
		}
		if function.InArray(ignoreApp, appName) {
			return filepath.SkipDir
		}

		storeItem := accessor.StoreAppDetail{
			Name:     appName,
			Version:  make(map[string]accessor.StoreAppVersionItem),
			Contents: make(map[string]string),
		}

		content, err := os.ReadFile(filepath.Join(appPath, "data.yml"))
		if err != nil {
			// 如果找不到 data.yml 文件则直接跳过这个应用
			slog.Debug("store onepanel sync not found data.yml", "name", appName)
			return filepath.SkipDir
		}
		yamlData := new(function.ConfigMap)
		err = yaml.Unmarshal(content, &yamlData)
		if err != nil {
			return err
		}
		descriptions := function.PluckMapWalk(yamlData.GetStringMapString("additionalProperties.description"), func(k string, v string) bool {
			return function.InArray([]string{
				"zh", "en",
			}, k)
		})
		storeItem.Description = yamlData.GetString("additionalProperties.shortDescZh")
		storeItem.Descriptions = map[string]string{
			define.LangZh: descriptions["zh"],
			define.LangEn: descriptions["en"],
		}
		storeItem.Tag = yamlData.GetStringSlice("additionalProperties.tags")
		storeItem.Website = yamlData.GetString("additionalProperties.website")
		storeItem.Title = yamlData.GetString("additionalProperties.name")

		resourcePath, _ := filepath.Rel(filepath.Dir(filepath.Dir(storePath)), appPath)
		r := time.Now().Unix()
		if _, err := os.Stat(filepath.Join(appPath, "logo.png")); err == nil {
			storeItem.Logo = fmt.Sprintf("image://%s/logo.png?r=%d", resourcePath, r)
		}

		if _, err := os.Stat(filepath.Join(appPath, "README.md")); err == nil {
			storeItem.Content = fmt.Sprintf("markdown-file://%s/README.md?r=%d", resourcePath, r)
			storeItem.Contents[define.LangZh] = fmt.Sprintf("markdown-file://%s/README.md?r=%d", resourcePath, r)
		}
		if _, err := os.Stat(filepath.Join(appPath, "README_en.md")); err == nil {
			storeItem.Contents[define.LangEn] = fmt.Sprintf("markdown-file://%s/README_en.md?r=%d", resourcePath, r)
		}

		err = filepath.WalkDir(appPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if path == appPath {
				return nil
			}
			if !d.IsDir() {
				return nil
			}
			versionName, _ := filepath.Rel(appPath, path)
			versionPath := filepath.Join(appPath, versionName)

			storeVersionItem := accessor.StoreAppVersionItem{
				Script:      map[string]string{},
				Environment: make([]types.EnvItem, 0),
				Name:        versionName,
			}

			storeVersionItem.ComposeFile = filepath.Join(resourcePath, versionName, "docker-compose.yml")

			content, err := os.ReadFile(filepath.Join(versionPath, "data.yml"))
			if err != nil {
				return err
			}
			yamlData := new(function.ConfigMap)
			err = yaml.Unmarshal(content, &yamlData)
			if err != nil {
				return err
			}
			if v := self.parseSetting(yamlData, "additionalProperties.formFields"); v != nil {
				storeVersionItem.Environment = append(storeVersionItem.Environment, v...)
			}

			for _, name := range []string{
				"install.sh", "upgrade.sh", "init.sh", "uninstall.sh",
			} {
				if _, err := os.Stat(filepath.Join(versionPath, "scripts", name)); err == nil {
					storeVersionItem.Script[name] = filepath.Join("scripts", name)
				}
			}

			storeVersionItem.Environment = onePanelEnvironment(appName, storeVersionItem.Environment)
			storeItem.Version[versionName] = storeVersionItem
			// 找到版本目录即可
			return filepath.SkipDir
		})

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

func (self OnePanel) parseSetting(getter *function.ConfigMap, root string) []types.EnvItem {
	result := make([]types.EnvItem, 0)
	fields := getter.GetSliceStringMapString(root)

	for index, field := range fields {
		labels := function.PluckMapWalk(getter.GetStringMapString(fmt.Sprintf("%s.%d.label", root, index)), func(k string, v string) bool {
			return function.InArray([]string{
				"zh", "en",
			}, k)
		})

		envItem := types.EnvItem{
			Label: field["labelZh"],
			Labels: map[string]string{
				define.LangZh: labels["zh"],
				define.LangEn: labels["en"],
			},
			Name:  field["envKey"],
			Value: field["default"],
			Rule: &types.EnvValueRule{
				Kind:   0,
				Option: make([]types.ValueItem, 0),
			},
		}
		envItem.Rule = types.NewEnvValueRule(field, func(item *types.EnvValueRule) {
			if (item.Kind&types.EnvValueTypeSelect) != 0 || (item.Kind&types.EnvValueTypeSelectMultiple) != 0 {
				item.Option = function.PluckArrayWalk(
					getter.GetSliceStringMapString(fmt.Sprintf("%s.%d.values", root, index)),
					func(i map[string]string) (types.ValueItem, bool) {
						return types.ValueItem{
							Name:  i["label"],
							Value: i["value"],
						}, true
					},
				)
			}
		})
		result = append(result, envItem)
	}
	// 如果包含了 PANEL_DB_TYPE 附加数据库其它参数
	if index, ok := function.IndexArrayWalk(result, func(item types.EnvItem) bool {
		return item.Name == "PANEL_DB_TYPE"
	}); ok {
		result = append(result[:index], append([]types.EnvItem{
			onePanelCommonEnv[define.StoreEnvDBHost], onePanelCommonEnv[define.StoreEnvDBPort],
		}, result[index:]...)...)
	}
	return result
}

var onePanelCommonEnv = map[int]types.EnvItem{
	define.StoreEnvContainerName: {
		Name:  "CONTAINER_NAME",
		Value: compose.PlaceholderAppName,
		Rule: &types.EnvValueRule{
			Kind: types.EnvValueRuleRequired | types.EnvValueTypeOnePanel,
		},
	},
	define.StoreEnvWebsiteDir: {
		Name:  "PANEL_WEBSITE_DIR",
		Value: "./wwwroot",
		Rule: &types.EnvValueRule{
			Kind: types.EnvValueRuleRequired | types.EnvValueTypeOnePanel,
		},
	},
	define.StoreEnvCodeDir: {
		Name:  "CODE_DIR",
		Value: "./www",
		Rule: &types.EnvValueRule{
			Kind: types.EnvValueRuleRequired | types.EnvValueTypeOnePanel,
		},
	},
	define.StoreEnvHostIp: {
		Name:  "HOST_IP",
		Value: "0.0.0.0",
		Rule: &types.EnvValueRule{
			Kind:   types.EnvValueTypeSelect | types.EnvValueTypeOnePanel,
			Option: types.NewValueItemWithArray("0.0.0.0", "127.0.0.1"),
		},
	},
	define.StoreEnvHostPort: {
		Name:  "PANEL_APP_PORT_HTTP",
		Value: "0",
		Rule: &types.EnvValueRule{
			Kind: types.EnvValueRuleRequired | types.EnvValueTypeOnePanel,
		},
	},
	define.StoreEnvContainerPort: {
		Name: "APP_PORT",
		Rule: &types.EnvValueRule{
			Kind: types.EnvValueRuleRequired | types.EnvValueTypeOnePanel,
		},
	},
	define.StoreEnvRunScript: {
		Name:  "EXEC_SCRIPT",
		Value: "",
		Rule: &types.EnvValueRule{
			Kind: types.EnvValueRuleRequired | types.EnvValueTypeOnePanel,
		},
	},
	define.StoreEnvDBHost: {
		Name:  "PANEL_DB_HOST",
		Value: "",
		Rule: &types.EnvValueRule{
			Kind: types.EnvValueRuleRequired | types.EnvValueTypeOnePanel,
		},
	},
	define.StoreEnvDBPort: {
		Name:  "PANEL_DB_PORT",
		Value: "3306",
		Rule: &types.EnvValueRule{
			Kind: types.EnvValueRuleRequired | types.EnvValueTypeNumber | types.EnvValueTypeOnePanel,
		},
	},
}

var onePanelDefaultEnv = map[string][]types.EnvItem{
	"node": {
		onePanelCommonEnv[define.StoreEnvCodeDir],
		onePanelCommonEnv[define.StoreEnvHostIp], onePanelCommonEnv[define.StoreEnvHostPort], onePanelCommonEnv[define.StoreEnvContainerPort], onePanelCommonEnv[define.StoreEnvRunScript],
		{
			Name:  "NODE_VERSION",
			Value: compose.PlaceholderAppVersion,
			Rule: &types.EnvValueRule{
				Kind: types.EnvValueRuleDisabled | types.EnvValueTypeOnePanel,
			},
		},
		{
			Name:  "CONTAINER_PACKAGE_URL",
			Value: "https://registry.npmjs.org/",
			Rule: &types.EnvValueRule{
				Kind: types.EnvValueTypeSelect | types.EnvValueTypeOnePanel,
				Option: types.NewValueItemWithArray(
					"https://registry.npmjs.org/",
					"https://registry.npmmirror.com/",
					"https://mirrors.cloud.tencent.com/npm/",
				),
			},
		},
		{
			Name:  "CUSTOM_SCRIPT",
			Value: "1",
			Rule: &types.EnvValueRule{
				Kind: types.EnvValueRuleDisabled | types.EnvValueTypeOnePanel,
			},
		},
	},
	"php": {
		onePanelCommonEnv[define.StoreEnvWebsiteDir], onePanelCommonEnv[define.StoreEnvHostPort],
		{
			Name:  "IMAGE_NAME",
			Value: fmt.Sprintf("%s-%s:%s", compose.PlaceholderAppName, compose.PlaceholderAppTaskName, compose.PlaceholderAppVersion),
			Rule: &types.EnvValueRule{
				Kind: types.EnvValueRuleDisabled | types.EnvValueTypeOnePanel,
			},
		},
	},
	"go": {
		onePanelCommonEnv[define.StoreEnvCodeDir],
		onePanelCommonEnv[define.StoreEnvHostIp], onePanelCommonEnv[define.StoreEnvHostPort], onePanelCommonEnv[define.StoreEnvContainerPort], onePanelCommonEnv[define.StoreEnvRunScript],
		{
			Name:  "GO_VERSION",
			Value: compose.PlaceholderAppVersion,
			Rule: &types.EnvValueRule{
				Kind: types.EnvValueRuleDisabled | types.EnvValueTypeOnePanel,
			},
		},
	},
	"java": {
		onePanelCommonEnv[define.StoreEnvCodeDir],
		onePanelCommonEnv[define.StoreEnvHostIp], onePanelCommonEnv[define.StoreEnvHostPort], onePanelCommonEnv[define.StoreEnvContainerPort], onePanelCommonEnv[define.StoreEnvRunScript],
		{
			Name:  "JAVA_VERSION",
			Value: compose.PlaceholderAppVersion,
			Rule: &types.EnvValueRule{
				Kind: types.EnvValueRuleDisabled | types.EnvValueTypeOnePanel,
			},
		},
	},
}

func onePanelEnvironment(appName string, environment []types.EnvItem) []types.EnvItem {
	itemList := append(append([]types.EnvItem{}, environment...), onePanelDefaultEnv[appName]...)
	itemList = append(itemList, onePanelCommonEnv[define.StoreEnvContainerName])
	result := make([]types.EnvItem, 0, len(itemList))
	indexMap := make(map[string]int, len(itemList))
	for _, item := range itemList {
		if index, ok := indexMap[item.Name]; ok {
			result[index] = item
		} else {
			indexMap[item.Name] = len(result)
			result = append(result, item)
		}
	}
	return result
}
