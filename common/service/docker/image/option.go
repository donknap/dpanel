package image

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
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
		self.imageBuildOption.Dockerfile = filepath.Base(self.imageBuildOption.Dockerfile)
		return nil
	}
}

func WithDockerFilePath(root, name string) Option {
	return func(self *Builder) error {
		if root == "" {
			root = "/"
		}
		if name == "" {
			name = "Dockerfile"
		}
		self.imageBuildOption.Dockerfile = filepath.Join(root, name)
		return nil
	}
}

func WithTag(name ...string) Option {
	return func(self *Builder) error {
		if name == nil {
			return errors.New("tag name is required")
		}
		self.imageBuildOption.Tags = append(self.imageBuildOption.Tags, name...)
		return nil
	}
}

func WithPlatform(item *docker.ImagePlatform) Option {
	return func(self *Builder) error {
		if item.Arch == "" || item.Type == "" {
			return nil
		}
		self.imageBuildOption.Platform = item.Type
		self.imageBuildOption.BuildArgs["TARGETARCH"] = function.Ptr(item.Arch)
		return nil
	}
}

func WithZipFilePath(path string) Option {
	return func(self *Builder) error {
		if path == "" {
			return nil
		}
		zipArchive, err := zip.OpenReader(path)
		if err != nil {
			return err
		}
		defer func() {
			_ = zipArchive.Close()
			_ = os.Remove(path)
		}()

		buf := new(bytes.Buffer)
		tarWriter := tar.NewWriter(buf)
		defer func() {
			_ = tarWriter.Close()
		}()
		trimPath := filepath.Dir(self.imageBuildOption.Dockerfile)
		self.imageBuildOption.Dockerfile = filepath.Base(self.imageBuildOption.Dockerfile)
		for _, zipFile := range zipArchive.File {
			if strings.HasPrefix(zipFile.Name, "__MACOSX") {
				continue
			}
			fileInfoHeader, err := tar.FileInfoHeader(zipFile.FileInfo(), "")
			fileInfoHeader.Name = strings.TrimPrefix(zipFile.Name, trimPath)

			if err != nil {
				return err
			}
			err = tarWriter.WriteHeader(fileInfoHeader)
			if err != nil {
				return err
			}
			zipFileReader, err := zipFile.Open()
			_, err = io.Copy(tarWriter, zipFileReader)
			if err != nil {
				return err
			}
		}
		self.buildContext = buf
		return nil
	}
}

func WithContext(ctx context.Context) Option {
	return func(self *Builder) error {
		self.ctx = ctx
		return nil
	}
}

func WithArgs(args ...docker.EnvItem) Option {
	return func(self *Builder) error {
		self.imageBuildOption.BuildArgs = make(map[string]*string)
		for _, arg := range args {
			self.imageBuildOption.BuildArgs[arg.Name] = &arg.Value
		}
		return nil
	}
}
