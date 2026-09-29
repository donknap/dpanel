package build

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/archive"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
)

type Option func(builder *Builder) error

func WithDockerFileContent(content []byte) Option {
	return func(self *Builder) error {
		if content == nil || len(content) == 0 {
			return nil
		}
		buf := new(bytes.Buffer)
		tarWriter := tar.NewWriter(buf)
		defer func() {
			_ = tarWriter.Close()
		}()
		if err := tarWriter.WriteHeader(&tar.Header{
			Name:    "Dockerfile",
			Size:    int64(len(content)),
			Mode:    int64(os.ModePerm),
			ModTime: time.Now(),
		}); err != nil {
			return err
		}
		if _, err := tarWriter.Write(content); err != nil {
			return err
		}
		self.buildContext = buf
		return nil
	}
}

func WithGitUrl(url string) Option {
	return func(self *Builder) error {
		if url == "" {
			return nil
		}
		self.imageBuildOption.RemoteContext = url
		return nil
	}
}

func WithDockerFilePath(path string) Option {
	return func(self *Builder) error {
		if path == "" {
			path = "Dockerfile"
		}
		self.imageBuildOption.Dockerfile = path
		return nil
	}
}

func WithTag(name ...string) Option {
	return func(self *Builder) error {
		if name == nil || len(name) == 0 {
			return define.ErrorImageTagEmpty
		}
		self.imageBuildOption.Tags = append(self.imageBuildOption.Tags, name...)
		return nil
	}
}

func WithPlatform(item *types.ImagePlatform) Option {
	return func(self *Builder) error {
		if item == nil || item.Arch == "" || item.Type == "" {
			return nil
		}
		self.imageBuildOption.Platform = item.Type
		self.imageBuildOption.BuildArgs["TARGETARCH"] = function.Ptr(item.Arch)
		return nil
	}
}

func WithZipFilePath(trimPath string, path string) Option {
	return func(self *Builder) error {
		if path == "" {
			return nil
		}
		tempDir, err := storage.Local{}.CreateTempDir("")
		if err != nil {
			return err
		}
		tarPath := filepath.Join(tempDir, "context.tar")
		if err = archive.ZipToTar(self.ctx, path, tarPath, archive.Option{Root: trimPath}); err != nil {
			_ = os.RemoveAll(tempDir)
			return err
		}
		contextFile, err := os.Open(tarPath)
		if err != nil {
			_ = os.RemoveAll(tempDir)
			return err
		}
		cleanupResult := make(chan error, 1)
		self.cleanupResults = append(self.cleanupResults, cleanupResult)
		self.buildContext = contextFile
		ctx := self.ctx
		go func() {
			select {
			case <-ctx.Done():
			case <-self.closed:
			}
			cleanupResult <- errors.Join(contextFile.Close(), os.RemoveAll(tempDir))
		}()
		return nil
	}
}

func WithSdk(sdk *docker.Client) Option {
	return func(self *Builder) error {
		self.sdk = sdk
		return nil
	}
}

func WithContext(ctx context.Context) Option {
	return func(self *Builder) error {
		self.ctx = ctx
		return nil
	}
}

func WithArgs(args ...types.EnvItem) Option {
	return func(self *Builder) error {
		self.imageBuildOption.BuildArgs = make(map[string]*string)
		for _, arg := range args {
			self.imageBuildOption.BuildArgs[arg.Name] = &arg.Value
		}
		return nil
	}
}
