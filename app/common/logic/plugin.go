package logic

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/donknap/dpanel/common/service/storage"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/patrickmn/go-cache"
	"gopkg.in/yaml.v3"
)

const (
	pluginMaxBytes   = 100 << 20
	pluginMaxEntries = 10000
)

var (
	pluginLock        sync.Mutex
	pluginNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)
)

type PluginSource struct {
	Source   string   `json:"source"`
	Name     string   `json:"name"`
	Mirrors  []string `json:"mirrors"`
	IsSystem bool     `json:"isSystem"`
}

var SystemPluginSource = PluginSource{
	Source:   "https://github.com/DPanel-dev/plugin",
	Name:     "DPanel-dev/plugin",
	IsSystem: true,
	Mirrors:  []string{"docker.io/dpanel/plugin:latest", "registry.cn-hangzhou.aliyuncs.com/dpanel/plugin:latest"},
}

type PluginItem struct {
	Source  string            `json:"source"`
	Name    string            `json:"name"`
	Version string            `json:"version"`
	Title   map[string]string `json:"title"`
}

type PluginSyncResult struct {
	Source    string `json:"source"`
	Transport string `json:"transport,omitempty"`
}

type Plugin struct{}

func (source PluginSource) ReadList() ([]PluginItem, error) {
	items, ok := storage.LoadCache[[]PluginItem](fmt.Sprintf(storage.CacheKeyPluginItems, source.Name))
	if ok {
		return items, nil
	}
	pluginLock.Lock()
	defer pluginLock.Unlock()
	items, ok = storage.LoadCache[[]PluginItem](fmt.Sprintf(storage.CacheKeyPluginItems, source.Name))
	if ok {
		return items, nil
	}
	namespace, repository, valid := strings.Cut(source.Name, "/")
	if !valid || !pluginNamePattern.MatchString(namespace) || !pluginNamePattern.MatchString(repository) {
		return nil, fmt.Errorf("invalid plugin source name %q", source.Name)
	}
	items, err := scanPlugins(filepath.Join(storage.Local{}.GetPluginPath(), namespace, repository), source.Name)
	if err != nil {
		return nil, err
	}
	storage.Cache.Set(fmt.Sprintf(storage.CacheKeyPluginItems, source.Name), items, cache.NoExpiration)
	return items, nil
}

func scanPlugins(root, source string) ([]PluginItem, error) {
	dirs, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	items := make([]PluginItem, 0)
	for _, dir := range dirs {
		if !dir.IsDir() || !pluginNamePattern.MatchString(dir.Name()) {
			continue
		}
		manifestPath := filepath.Join(root, dir.Name(), "manifest.yaml")
		info, err := os.Lstat(manifestPath)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
			return nil, fmt.Errorf("invalid manifest in %s", dir.Name())
		}
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			return nil, err
		}
		var manifest struct {
			Plugin struct {
				Name    string            `yaml:"name"`
				Version string            `yaml:"version"`
				Title   map[string]string `yaml:"title"`
			} `yaml:"plugin"`
		}
		if err := yaml.Unmarshal(data, &manifest); err != nil || manifest.Plugin.Name != dir.Name() || manifest.Plugin.Version == "" {
			return nil, fmt.Errorf("invalid plugin manifest in %s", dir.Name())
		}
		compose, err := os.Lstat(filepath.Join(root, dir.Name(), "docker-compose.yaml"))
		if err != nil || !compose.Mode().IsRegular() {
			return nil, fmt.Errorf("plugin %s has no regular docker-compose.yaml", dir.Name())
		}
		items = append(items, PluginItem{Source: source, Name: dir.Name(), Version: manifest.Plugin.Version, Title: manifest.Plugin.Title})
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no plugins found in %s", root)
	}
	return items, nil
}

func (Plugin) Sync(ctx context.Context, source PluginSource) (PluginSyncResult, error) {
	pluginLock.Lock()
	defer pluginLock.Unlock()
	result := PluginSyncResult{Source: source.Source}
	namespace, repository, ok := strings.Cut(source.Name, "/")
	if !ok || !pluginNamePattern.MatchString(namespace) || !pluginNamePattern.MatchString(repository) {
		return result, fmt.Errorf("invalid plugin source name %q", source.Name)
	}
	root := storage.Local{}.GetPluginPath()
	if err := os.MkdirAll(root, 0700); err != nil {
		return result, err
	}
	var failures []string
	for index := 0; index <= len(source.Mirrors); index++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		transport := "git"
		if index < len(source.Mirrors) {
			transport = source.Mirrors[index]
		}
		stage, err := os.MkdirTemp(root, ".download-*")
		if err != nil {
			return result, err
		}
		if index < len(source.Mirrors) {
			err = (Store{}).SyncByOci(transport, stage, pluginMaxBytes, pluginMaxEntries, OciOption{
				SyncOption: SyncOption{
					Context: ctx,
				},
				Platform: v1.Platform{OS: "linux", Architecture: "amd64"},
				ExpectedLabels: map[string]string{
					"com.dpanel.plugin.source": source.Source,
					"com.dpanel.plugin.root":   "/",
				},
			})
		} else {
			err = (Store{}).SyncByGit(source.Source, stage, GitOption{
				SyncOption: SyncOption{
					Context: ctx,
				},
				TempDownloadPath: stage,
				RemoveGit:        true,
				MaxBytes:         pluginMaxBytes,
			})
		}
		var items []PluginItem
		if err == nil {
			items, err = scanPlugins(stage, source.Name)
		}
		if err == nil {
			if err = installPluginSource(stage, namespace, repository); err != nil {
				_ = os.RemoveAll(stage)
				return result, fmt.Errorf("install plugin source: %w", err)
			}
			storage.Cache.Set(fmt.Sprintf(storage.CacheKeyPluginItems, source.Name), items, cache.NoExpiration)
			result.Transport = transport
			return result, nil
		}
		if cleanupErr := os.RemoveAll(stage); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("remove temporary plugin source: %w", cleanupErr))
		}
		failures = append(failures, fmt.Sprintf("%s: %v", transport, err))
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
	}
	return result, errors.New(strings.Join(failures, "; "))
}

func installPluginSource(stage, namespace, repository string) error {
	parent := filepath.Join(storage.Local{}.GetPluginPath(), namespace)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return err
	}
	target := filepath.Join(parent, repository)
	backup := filepath.Join(parent, "."+repository+"-previous")
	if _, err := os.Lstat(backup); err == nil {
		if _, targetErr := os.Lstat(target); errors.Is(targetErr, os.ErrNotExist) {
			if err := os.Rename(backup, target); err != nil {
				return fmt.Errorf("restore previous source: %w", err)
			}
		} else if targetErr == nil {
			if err := os.RemoveAll(backup); err != nil {
				return fmt.Errorf("remove previous source backup: %w", err)
			}
		} else {
			return targetErr
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	previous := false
	if _, err := os.Lstat(target); err == nil {
		if err := os.Rename(target, backup); err != nil {
			return err
		}
		previous = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(stage, target); err != nil {
		if previous {
			if restoreErr := os.Rename(backup, target); restoreErr != nil {
				return fmt.Errorf("install: %v; restore: %w", err, restoreErr)
			}
		}
		return err
	}
	if previous {
		if err := os.RemoveAll(backup); err != nil {
			slog.Warn("remove previous plugin source backup", "path", backup, "error", err)
		}
	}
	return nil
}
