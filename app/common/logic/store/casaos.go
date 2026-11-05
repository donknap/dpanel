package store

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker/types"
	sourcesync "github.com/donknap/dpanel/common/service/sync"
	"github.com/donknap/dpanel/common/types/define"
	"gopkg.in/yaml.v3"
)

type CasaOs struct{ source }

func (self CasaOs) Sync(ctx context.Context, sourceURL string) ([]accessor.StoreAppDetail, error) {
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
	if err = sourcesync.ByZip(ctx, sourceURL, stagePath, stageRoot, sourcesync.ZipOption{Root: "Apps"}); err != nil {
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

func (self CasaOs) GetList() ([]accessor.StoreAppDetail, error) {
	storePath := filepath.Join(self.path, "Apps")
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

		storeItem := accessor.StoreAppDetail{
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
