package logic

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/donknap/dpanel/app/common/logic/onepanel"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"gopkg.in/yaml.v3"
)

func (self Store) GetAppByOnePanel(storePath string) ([]accessor.StoreAppItem, error) {
	if !filepath.IsAbs(storePath) {
		storePath = filepath.Join(storage.Local{}.GetStorePath(), storePath, "apps")
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

		// 忽略不支持的应用
		ignoreApp := []string{
			"php5", "php7", "php8",
		}
		if function.InArray(ignoreApp, appName) {
			return filepath.SkipDir
		}

		storeItem := accessor.StoreAppItem{
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
			if v := self.parseOnePanelSetting(yamlData, "additionalProperties.formFields"); v != nil {
				storeVersionItem.Environment = append(storeVersionItem.Environment, v...)
			}

			for _, name := range []string{
				"install.sh", "upgrade.sh", "init.sh", "uninstall.sh",
			} {
				if _, err := os.Stat(filepath.Join(versionPath, "scripts", name)); err == nil {
					storeVersionItem.Script[name] = filepath.Join("scripts", name)
				}
			}

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

func (self Store) parseOnePanelSetting(getter *function.ConfigMap, root string) []types.EnvItem {
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
		envItem.Rule = self.ParseSettingField(field, func(item *types.EnvValueRule) {
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
			onepanel.CommonEnv[define.StoreEnvDBHost], onepanel.CommonEnv[define.StoreEnvDBPort],
		}, result[index:]...)...)
	}
	return result
}
