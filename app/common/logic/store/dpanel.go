package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/storage"
	sourcesync "github.com/donknap/dpanel/common/service/sync"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"gopkg.in/yaml.v3"
)

type DPanel struct{ source }

func (self DPanel) Sync(ctx context.Context, sourceURL string) ([]accessor.StoreAppDetail, error) {
	storeRow := self.row
	if sourceURL == "" {
		return nil, os.ErrPermission
	}
	storeRootPath := function.SafePathJoin(storage.Local{}.GetStorePath(), storeRow.Name)
	appList := make([]accessor.StoreAppDetail, 0)
	var err error
	if err = os.MkdirAll(storage.Local{}.GetStorePath(), 0755); err != nil {
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
	stagePath := filepath.Join(stageRoot, storeRow.Name)
	repositoryURL, _, _ := strings.Cut(storeRow.Setting.Url, "#")
	// 镜像发布标签使用仓库页面地址，而不是带 .git 的克隆地址。
	repositoryURL = strings.TrimSuffix(repositoryURL, ".git")
	if sourcesync.IsGit(sourceURL) {
		err = sourcesync.ByGit(ctx, sourceURL, stagePath, stageRoot, sourcesync.GitOption{
			RemoveGit: true,
			MaxBytes:  100 << 20,
		})
	} else if sourcesync.IsOci(sourceURL) {
		err = sourcesync.ByOci(ctx, sourceURL, stagePath, stageRoot, sourcesync.OciOption{
			Platform:     v1.Platform{OS: "linux", Architecture: "amd64"},
			DockerClient: self.dockerClient,
			ExpectedLabels: map[string]string{
				"com.dpanel.plugin.source": repositoryURL,
				"com.dpanel.plugin.root":   "/",
			},
		})
	} else {
		return nil, fmt.Errorf("unsupported store source address")
	}
	if err != nil {
		return nil, err
	}
	candidate := self
	candidate.path = stagePath
	appList, err = candidate.GetList()
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	storeSourceMutex.Lock()
	defer storeSourceMutex.Unlock()
	backupPath := filepath.Join(stageRoot, "previous")
	if err = os.Rename(storeRootPath, backupPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	previousAppList := storeRow.Setting.Apps
	previousTotal := storeRow.Setting.Total
	previousUpdatedAt := storeRow.Setting.UpdatedAt
	err = os.Rename(stagePath, storeRootPath)
	if err == nil {
		storeRow.Setting.Apps = appList
		storeRow.Setting.Total = len(appList)
		storeRow.Setting.UpdatedAt = time.Now().Unix()
		err = dao.Store.Save(storeRow)
	}
	if err != nil {
		storeRow.Setting.Apps = previousAppList
		storeRow.Setting.Total = previousTotal
		storeRow.Setting.UpdatedAt = previousUpdatedAt
		if restoreErr := os.Rename(storeRootPath, stagePath); restoreErr != nil && !errors.Is(restoreErr, os.ErrNotExist) {
			err = errors.Join(err, fmt.Errorf("store recovery directory %s: %w", stageRoot, restoreErr))
			stageRoot = ""
		} else if restoreErr := os.Rename(backupPath, storeRootPath); restoreErr != nil && !errors.Is(restoreErr, os.ErrNotExist) {
			err = errors.Join(err, fmt.Errorf("store recovery directory %s: %w", stageRoot, restoreErr))
			stageRoot = ""
		}
		return nil, err
	}
	return appList, nil
}

func (self DPanel) GetList() ([]accessor.StoreAppDetail, error) {
	storePath := self.path
	directoryList, err := os.ReadDir(storePath)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(storePath)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	appList := make([]accessor.StoreAppDetail, 0)
	for _, directory := range directoryList {
		if !directory.IsDir() || !function.IsSafeString(directory.Name(), function.StringRuleName) {
			continue
		}
		appRoot, err := root.OpenRoot(directory.Name())
		if err != nil {
			return nil, err
		}
		info, err := appRoot.Lstat("manifest.yaml")
		if errors.Is(err, os.ErrNotExist) {
			_ = appRoot.Close()
			continue
		}
		if err != nil {
			_ = appRoot.Close()
			return nil, err
		}
		if !info.Mode().IsRegular() || info.Size() > 1<<20 {
			_ = appRoot.Close()
			return nil, os.ErrPermission
		}
		content, err := appRoot.ReadFile("manifest.yaml")
		_ = appRoot.Close()
		if err != nil {
			return nil, err
		}
		var manifest struct {
			Version string `yaml:"version"`
			Plugin  struct {
				Name        string            `yaml:"name"`
				Version     string            `yaml:"version"`
				Title       map[string]string `yaml:"title"`
				DefaultLang string            `yaml:"defaultLang"`
				Bootstrap   string            `yaml:"bootstrap"`
			} `yaml:"plugin"`
		}
		if err = yaml.Unmarshal(content, &manifest); err != nil {
			return nil, fmt.Errorf("store app %s manifest: %w", directory.Name(), err)
		}
		name, version := manifest.Plugin.Name, manifest.Plugin.Version
		if manifest.Version != "1.0" || name != directory.Name() || version == "" || manifest.Plugin.Bootstrap != "compose" {
			return nil, fmt.Errorf("invalid store app manifest in %s", directory.Name())
		}
		title := manifest.Plugin.Title[manifest.Plugin.DefaultLang]
		if title == "" {
			title = name
		}
		composePath := filepath.Join(name, "docker-compose.yaml")
		info, err = root.Lstat(composePath)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > 1<<20 {
			return nil, fmt.Errorf("invalid store app YAML file in %s", name)
		}
		appList = append(appList, accessor.StoreAppDetail{
			Name:   name,
			Title:  title,
			Titles: manifest.Plugin.Title,
			Version: map[string]accessor.StoreAppVersionItem{
				version: {
					Name:        version,
					ComposeFile: path.Join(filepath.Base(storePath), name, "docker-compose.yaml"),
					Default:     true,
				},
			},
		})
	}
	return appList, nil
}

func (self DPanel) PreInstall(detail accessor.StoreAppInstallDetail) error {
	return ErrUnsupported
}
