package image

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"errors"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"io"
	"os"
	"strings"
	"time"
)

type Option func(builder *Builder) error

func WithDockerFileContent(content []byte) Option {
	return func(self *Builder) error {
		if content == nil {
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
			return nil
		}
		self.imageBuildOption.Dockerfile = path
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
		if item == nil {
			return nil
		}
		self.imageBuildOption.Platform = item.Type
		self.imageBuildOption.BuildArgs["TARGETARCH"] = function.PtrString(item.Arch)
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

		for _, zipFile := range zipArchive.File {
			if strings.HasPrefix(zipFile.Name, "__MACOSX") {
				continue
			}
			fileInfoHeader, err := tar.FileInfoHeader(zipFile.FileInfo(), "")
			fileInfoHeader.Name = zipFile.Name

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
