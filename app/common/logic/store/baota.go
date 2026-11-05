package store

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/compose"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/storage"
	sourcesync "github.com/donknap/dpanel/common/service/sync"
	"github.com/donknap/dpanel/common/types/define"
	"gopkg.in/yaml.v3"
)

type BaoTa struct{ source }

func (self BaoTa) Sync(ctx context.Context, sourceURL string) ([]accessor.StoreAppDetail, error) {
	baseURL, err := url.Parse(sourceURL)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Dir(self.path), 0755); err != nil {
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
		if err := os.RemoveAll(stageRoot); err != nil {
			slog.Warn("remove store sync temporary directory", "error", err)
		}
	}()
	for _, fileName := range []string{"apps.json", "dkapp_ico.zip"} {
		filePath := filepath.Join(stageRoot, fileName)
		if err = sourcesync.ByUrl(ctx, baseURL.JoinPath(fileName).String(), filePath, sourcesync.UrlOption{}); err != nil {
			return nil, err
		}
		if fileName == "apps.json" {
			content, err := os.ReadFile(filePath)
			if err != nil {
				return nil, err
			}
			var appList []interface{}
			if err = json.Unmarshal(content, &appList); err != nil {
				return nil, err
			}
		} else {
			archive, err := zip.OpenReader(filePath)
			if err != nil {
				return nil, err
			}
			if err = archive.Close(); err != nil {
				return nil, err
			}
		}
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	storeSourceMutex.Lock()
	defer storeSourceMutex.Unlock()
	if _, err = function.CopyDir(self.path, stageRoot, function.CopyDirOption{Mode: function.CopyModeOverwrite}); err != nil {
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

func (self BaoTa) GetList() ([]accessor.StoreAppDetail, error) {
	storePath := self.path
	urls, err := url.Parse(self.row.Setting.Url)
	if err != nil {
		return nil, err
	}

	appsJson, err := os.ReadFile(filepath.Join(storePath, "apps.json"))
	if err != nil {
		return nil, err
	}
	logoZip, err := zip.OpenReader(filepath.Join(storePath, "dkapp_ico.zip"))
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = logoZip.Close()
	}()

	var jsonData []interface{}
	err = json.Unmarshal(appsJson, &jsonData)
	if err != nil {
		return nil, err
	}

	r := time.Now().Unix()

	result := make([]accessor.StoreAppDetail, 0)
	for _, item := range jsonData {
		itemData, err := yaml.Marshal(item)
		if err != nil {
			continue
		}
		config := new(function.ConfigMap)
		err = yaml.Unmarshal(itemData, &config)
		if err != nil {
			continue
		}
		storeItem := accessor.StoreAppDetail{
			Title:    config.GetString("apptitle"),
			Name:     config.GetString("appname"),
			Logo:     "",
			Contents: map[string]string{},
			Descriptions: map[string]string{
				define.LangZh: config.GetString("appdesc"),
			},
			Tag: []string{
				config.GetString("apptype"), config.GetString("appTypeCN"),
			},
			Website: config.GetString("home"),
			Version: make(map[string]accessor.StoreAppVersionItem),
		}
		storeItemPath := function.SafePathJoin(filepath.Join(storePath, "apps"), storeItem.Name)
		resourceStoreItemPath, _ := filepath.Rel(storage.Local{}.GetStorePath(), storeItemPath)

		_ = os.MkdirAll(storeItemPath, os.ModePerm)

		if logo, _, ok := function.PluckArrayItemWalk(logoZip.File, func(item *zip.File) bool {
			return fmt.Sprintf("dkapp_ico/ico-dkapp_%s.png", storeItem.Name) == item.Name
		}); ok {
			if reader, err := logo.Open(); err == nil {
				if f, err := os.OpenFile(filepath.Join(storeItemPath, filepath.Base(logo.Name)), os.O_CREATE|os.O_TRUNC|os.O_RDWR, logo.Mode()); err == nil {
					_, err := io.Copy(f, reader)
					_ = f.Close()
					if err != nil {
						slog.Debug("store bt sync copy icon", "err", err)
					} else {
						storeItem.Logo = fmt.Sprintf("image://%s/%s?r=%d", strings.ReplaceAll(resourceStoreItemPath, string(filepath.Separator), "/"), filepath.Base(logo.Name), r)
					}
				}
				_ = reader.Close()
			}
		}

		environment := make([]types.EnvItem, 0)
		if envList := config.GetSliceStringMapString("env"); !function.IsEmptyArray(envList) {
			for _, field := range envList {
				envItem := types.EnvItem{
					Label: "",
					Labels: map[string]string{
						define.LangZh: field["desc"],
					},
					Name:  strings.ToUpper(field["key"]),
					Value: field["default"],
				}
				envItem.Rule = types.NewEnvValueRule(field, nil)
				environment = append(environment, envItem)
			}
		}

		environment = baoTaEnvironment(environment)
		if version := config.GetSliceStringMapString("appversion"); !function.IsEmptyArray(version) && len(version) > 0 {
			firstVersionName := ""
			for i, versionItem := range version {
				mVersion := versionItem["m_version"]
				sVersion := config.GetStringSlice(fmt.Sprintf("appversion.%d.s_version", i))
				if function.IsEmptyArray(sVersion) {
					storeItem.Version[mVersion] = accessor.StoreAppVersionItem{
						Name:        mVersion,
						ComposeFile: filepath.Join(resourceStoreItemPath, "docker-compose.yml"),
						Environment: environment,
						Script:      make(map[string]string),
						Download:    urls.JoinPath("templates", storeItem.Name+".zip").String(),
						Default:     true,
					}
					firstVersionName = mVersion
				} else {
					for _, s := range sVersion {
						versionName := fmt.Sprintf("%s.%s", mVersion, s)
						if firstVersionName == "" {
							storeItem.Version[versionName] = accessor.StoreAppVersionItem{
								Name:        versionName,
								ComposeFile: filepath.Join(resourceStoreItemPath, "docker-compose.yml"),
								Environment: environment,
								Script:      make(map[string]string),
								Download:    urls.JoinPath("templates", storeItem.Name+".zip").String(),
								Default:     true,
							}
							firstVersionName = versionName
						} else {
							storeItem.Version[versionName] = accessor.StoreAppVersionItem{
								Name: versionName,
								Ref:  firstVersionName,
							}
						}
					}
				}
			}
		}

		result = append(result, storeItem)
	}
	return result, nil
}

var baoTaCommonEnv = map[int]types.EnvItem{
	define.StoreEnvContainerName: {
		Name:  "CONTAINER_NAME",
		Value: compose.PlaceholderAppName,
		Rule: &types.EnvValueRule{
			Kind: types.EnvValueRuleRequired | types.EnvValueTypeBaoTa,
		},
	},
	define.StoreEnvWebsiteDir: {
		Name:  "APP_PATH",
		Value: "./",
		Rule: &types.EnvValueRule{
			Kind: types.EnvValueRuleRequired | types.EnvValueTypeBaoTa,
		},
	},
	define.StoreEnvVersion: {
		Name:  "VERSION",
		Value: compose.PlaceholderAppVersion,
		Rule: &types.EnvValueRule{
			Kind: types.EnvValueRuleDisabled | types.EnvValueTypeBaoTa,
		},
	},
	define.StoreEnvHostIp: {
		Name:  "HOST_IP",
		Value: "0.0.0.0",
		Rule: &types.EnvValueRule{
			Kind:   types.EnvValueTypeSelect | types.EnvValueTypeBaoTa,
			Option: types.NewValueItemWithArray("0.0.0.0", "127.0.0.1"),
		},
	},
	define.StoreEnvLimitCpu: {
		Name:  "CPUS",
		Value: "0",
		Rule: &types.EnvValueRule{
			Kind: types.EnvValueRuleRequired | types.EnvValueTypeBaoTa,
		},
	},
	define.StoreEnvLimitMemory: {
		Name:  "MEMORY_LIMIT",
		Value: "0",
		Rule: &types.EnvValueRule{
			Kind: types.EnvValueRuleRequired | types.EnvValueTypeBaoTa,
		},
	},
}

func baoTaEnvironment(environment []types.EnvItem) []types.EnvItem {
	result := append([]types.EnvItem{}, environment...)
	for index, item := range result {
		if value, _, ok := function.PluckMapItemWalk(baoTaCommonEnv, func(_ int, value types.EnvItem) bool { return value.Name == item.Name }); ok {
			item.Value = value.Value
			if item.Rule == nil {
				item.Rule = &types.EnvValueRule{}
			} else {
				rule := *item.Rule
				item.Rule = &rule
			}
			item.Rule.Kind |= value.Rule.Kind
			item.Rule.Option = value.Rule.Option
		}
		result[index] = item
	}
	return result
}

func (self BaoTa) PreInstall(detail accessor.StoreAppInstallDetail) error {
	app, version, err := GetApp(self.row, detail)
	if err != nil {
		return err
	}
	mirrorList := make([]string, 0, len(self.row.Setting.Mirror))
	for _, mirror := range self.row.Setting.Mirror {
		mirrorURL, err := url.Parse(mirror)
		if err != nil {
			return err
		}
		mirrorList = append(mirrorList, mirrorURL.JoinPath("templates", app.Name+".zip").String())
	}
	targetPath := filepath.Join(storage.Local{}.GetStorePath(), filepath.Dir(function.SlashPathToSystem(version.ComposeFile)))
	if filepath.Clean(targetPath) == filepath.Clean(self.path) {
		return os.ErrPermission
	}
	if err = os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(self.tempPath, 0700); err != nil {
		return err
	}
	stageRoot, err := os.MkdirTemp(self.tempPath, ".sync-*")
	if err != nil {
		return err
	}
	defer func() {
		if stageRoot != "" {
			if err := os.RemoveAll(stageRoot); err != nil {
				slog.Warn("remove store install temporary directory", "error", err)
			}
		}
	}()
	ctx := context.Background()
	sourceList := append(mirrorList, version.Download)
	for index, address := range sourceList {
		stagePath := filepath.Join(stageRoot, fmt.Sprint(index))
		err = sourcesync.ByZip(ctx, address, stagePath, stageRoot, sourcesync.ZipOption{Root: app.Name, StripRoot: true})
		if err == nil {
			var root *os.Root
			root, err = os.OpenRoot(stagePath)
			if err == nil {
				var info os.FileInfo
				info, err = root.Stat(filepath.Base(version.ComposeFile))
				_ = root.Close()
				if err == nil && (!info.Mode().IsRegular() || info.Size() == 0) {
					err = fmt.Errorf("store app %q version %q: empty or invalid compose YAML", detail.AppName, detail.Version)
				}
			}
		}
		if err != nil {
			continue
		}
		storeSourceMutex.Lock()
		defer storeSourceMutex.Unlock()
		// 安装包覆盖同名文件，原目录中的图标等额外文件保留。
		_, err = function.CopyDir(stagePath, targetPath, function.CopyDirOption{Mode: function.CopyModeSkip})
		var pathErr *os.PathError
		if errors.As(err, &pathErr) && pathErr.Path == targetPath && errors.Is(err, os.ErrNotExist) {
			err = nil
		}
		if err != nil {
			return err
		}
		backupPath := filepath.Join(stageRoot, "previous")
		if err = os.Rename(targetPath, backupPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err = os.Rename(stagePath, targetPath); err != nil {
			if restoreErr := os.Rename(backupPath, targetPath); restoreErr != nil && !errors.Is(restoreErr, os.ErrNotExist) {
				err = errors.Join(err, fmt.Errorf("store recovery directory %s: %w", stageRoot, restoreErr))
				stageRoot = ""
			}
			return err
		}
		return nil
	}
	return err
}
