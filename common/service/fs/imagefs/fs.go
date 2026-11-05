package imagefs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/versions"
	"github.com/donknap/dpanel/common/service/agent/factor"
	"github.com/donknap/dpanel/common/service/docker"
	dockerTypes "github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/fs/afs"
	"github.com/donknap/dpanel/common/service/fs/dockerfs"
	"github.com/donknap/dpanel/common/service/storage"
	filedata "github.com/donknap/dpanel/common/types/fs"
)

var _ afs.ReadFs = (*Fs)(nil)

type Fs struct {
	ctx           context.Context
	cancel        context.CancelFunc
	name          string
	dockerFs      *dockerfs.Fs
	entryMap      map[string]*tarEntry
	directoryPath string
	archivePath   string
	mutex         sync.Mutex
	closed        bool
	cleanup       func() error
	stopCleanup   func() bool
}

// New 绑定调用方的 Docker 连接；temporaryDir 存放本实例的临时文件，不使用系统临时目录。
func New(ctx context.Context, dockerSdk *docker.Client, imageID, temporaryDir string) (_ *Fs, err error) {
	if dockerSdk == nil || dockerSdk.Client == nil {
		return nil, errors.New("docker client is required for image filesystem")
	}
	if temporaryDir == "" {
		return nil, errors.New("temporary directory is required for image filesystem")
	}
	ctx, cancel := context.WithCancel(ctx)
	self := &Fs{ctx: ctx, cancel: cancel}
	self.cleanup = sync.OnceValue(func() error {
		self.mutex.Lock()
		defer self.mutex.Unlock()
		self.closed = true
		var err error
		if self.archivePath != "" {
			err = errors.Join(err, os.Remove(self.archivePath))
		}
		if self.directoryPath != "" {
			err = errors.Join(err, os.Remove(self.directoryPath))
		}
		return err
	})
	defer func() {
		if err != nil {
			cancel()
			err = errors.Join(err, self.cleanup())
		}
	}()
	imageInfo, err := dockerSdk.Client.ImageInspect(ctx, imageID)
	if err != nil {
		return nil, err
	}
	self.name = imageInfo.ID
	version, err := dockerSdk.Client.ServerVersion(ctx)
	if err != nil {
		return nil, err
	}
	if version.Os == "linux" && versions.GreaterThanOrEqualTo(version.APIVersion, "1.48") &&
		versions.GreaterThanOrEqualTo(dockerSdk.Client.ClientVersion(), "1.48") &&
		(version.Arch == "amd64" || version.Arch == "arm64" || version.Arch == "arm") {
		taskSdk := &docker.Client{Name: dockerSdk.Name, Client: dockerSdk.Client, Ctx: ctx, DockerEnv: dockerSdk.DockerEnv}
		var containerName string
		containerName, err = factor.NewExplorer(taskSdk, factor.ExplorerCreateOption{
			Volumes: []dockerTypes.VolumeItem{
				{Host: imageInfo.ID, Dest: factor.ImageExplorerMountPath, Type: "image", Permission: "readonly"},
			},
		})
		if err != nil {
			return nil, err
		}
		self.dockerFs, err = dockerfs.New(
			dockerfs.WithName(imageInfo.ID), dockerfs.WithDockerSdk(taskSdk),
			dockerfs.WithProxyContainer(containerName), dockerfs.WithRoot(factor.ImageExplorerMountPath),
		)
	} else {
		cacheKey := fmt.Sprintf(storage.CacheKeyImageRootFs, dockerSdk.Name+"\x00"+dockerSdk.Client.DaemonHost()+"\x00"+imageInfo.ID)
		if cache, ok := storage.LoadCache[map[string]filedata.FileData](cacheKey); ok {
			self.entryMap = make(map[string]*tarEntry, len(cache))
			for name, info := range cache {
				self.entryMap[name] = &tarEntry{info: info}
			}
		} else {
			err = self.loadTar(dockerSdk.Client, temporaryDir)
			if err == nil {
				cache := make(map[string]filedata.FileData, len(self.entryMap))
				for name, entry := range self.entryMap {
					cache[name] = entry.info
				}
				storage.Cache.Set(cacheKey, cache, time.Hour)
			}
		}
	}
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	self.stopCleanup = context.AfterFunc(ctx, func() {
		if err := self.cleanup(); err != nil {
			slog.Warn("release image filesystem", "image", self.name, "error", err)
		}
	})
	return self, nil
}

func (self *Fs) Name() string { return self.name }

func (self *Fs) WorkingDir() string { return "/" }

func (self *Fs) RootDirs() ([]string, error) { return []string{"/"}, nil }

func (self *Fs) Destroy() error {
	self.stopCleanup()
	self.cancel()
	return self.cleanup()
}

func (self *Fs) List(name string) ([]*filedata.FileData, error) {
	self.mutex.Lock()
	defer self.mutex.Unlock()
	info, err := self.pathInfo(name)
	if err != nil {
		return nil, err
	}
	if !info.IsDir {
		return nil, &os.PathError{Op: "list", Path: name, Err: errors.New("not a directory")}
	}
	entryList, err := self.list(info.Path)
	if err != nil {
		return nil, err
	}
	for _, entry := range entryList {
		entry.Path = path.Join(name, entry.Name)
	}
	slices.SortFunc(entryList, compareFileData)
	return entryList, nil
}

// pathInfo 在镜像自己的根目录内解释链接，不把镜像中的绝对链接解释成助手容器路径。
func (self *Fs) pathInfo(name string) (*filedata.FileData, error) {
	if self.closed {
		return nil, os.ErrClosed
	}
	if err := self.ctx.Err(); err != nil {
		return nil, err
	}
	if !path.IsAbs(name) || path.Clean(name) != name || strings.ContainsRune(name, 0) {
		return nil, &os.PathError{Op: "stat", Path: name, Err: os.ErrInvalid}
	}
	current := "/"
	componentList := strings.Split(strings.TrimPrefix(name, "/"), "/")
	if name == "/" {
		componentList = nil
	}
	linkCount := 0
	for {
		if len(componentList) > 0 {
			current = path.Join(current, componentList[0])
			componentList = componentList[1:]
		}
		info, err := self.entryInfo(current)
		if err != nil {
			return nil, err
		}
		if info.IsSymlink {
			linkCount++
			if linkCount > 40 || info.LinkName == "" || strings.ContainsRune(info.LinkName, 0) {
				return nil, &os.PathError{Op: "stat", Path: name, Err: errors.New("invalid symbolic link chain")}
			}
			target := info.LinkName
			if !path.IsAbs(target) {
				target = path.Join(path.Dir(current), target)
			}
			target = path.Join(target, path.Join(componentList...))
			componentList = strings.Split(strings.TrimPrefix(target, "/"), "/")
			if target == "/" {
				componentList = nil
			}
			current = "/"
			continue
		}
		if len(componentList) == 0 {
			return info, nil
		}
		if !info.IsDir {
			return nil, &os.PathError{Op: "stat", Path: name, Err: errors.New("not a directory")}
		}
	}
}

func (self *Fs) entryInfo(name string) (*filedata.FileData, error) {
	if self.dockerFs == nil {
		if entry, ok := self.entryMap[name]; ok {
			info := entry.info
			return &info, nil
		}
		return nil, &os.PathError{Op: "stat", Path: name, Err: os.ErrNotExist}
	}
	if name == "/" {
		info, _, err := self.dockerFs.LstatIfPossible(name)
		if err != nil {
			return nil, err
		}
		data := *info.Sys().(*filedata.FileData)
		return &data, nil
	}
	entryList, err := self.dockerFs.List(path.Dir(name))
	if err != nil {
		return nil, err
	}
	for _, entry := range entryList {
		if entry.Path == name {
			return entry, nil
		}
	}
	return nil, &os.PathError{Op: "stat", Path: name, Err: os.ErrNotExist}
}

func (self *Fs) list(name string) ([]*filedata.FileData, error) {
	if self.dockerFs != nil {
		return self.dockerFs.List(name)
	}
	result := make([]*filedata.FileData, 0)
	for entryPath, entry := range self.entryMap {
		if entryPath != name && path.Dir(entryPath) == name {
			info := entry.info
			result = append(result, &info)
		}
	}
	return result, nil
}

func compareFileData(left, right *filedata.FileData) int {
	if left.IsDir != right.IsDir {
		if left.IsDir {
			return -1
		}
		return 1
	}
	return strings.Compare(left.Path, right.Path)
}
