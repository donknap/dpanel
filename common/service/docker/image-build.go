package docker

import (
	"archive/tar"
	"archive/zip"
	"context"
	"errors"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"io"
	"log/slog"
	"os"
	"strings"
)

type imageBuildBuilder struct {
	//zipFileReader *zip.Reader
	imageBuildOption  types.ImageBuildOptions
	zipFilePath       string
	dockerFileContent []byte
	dockerSdk         *client.Client
}

func (self *imageBuildBuilder) withSdk(sdk *client.Client) {
	self.dockerSdk = sdk
}

func (self *imageBuildBuilder) WithDockerFileContent(content []byte) {
	self.dockerFileContent = content
}

func (self *imageBuildBuilder) WithGitUrl(git string) {
	self.imageBuildOption.RemoteContext = git
}

func (self *imageBuildBuilder) WithLabel(name string, value string) {
	self.imageBuildOption.Labels[name] = value
}

func (self *imageBuildBuilder) WithDockerFilePath(path string) {
	self.imageBuildOption.Dockerfile = path
}

func (self *imageBuildBuilder) WithTag(name string) {
	self.imageBuildOption.Tags = append(self.imageBuildOption.Tags, name)
}

// fileInfo, _ := file.Stat()
// zipReader, _ := zip.NewReader(file, fileInfo.Size())
//func (self *imageBuildBuilder) WithZipFileRead(reader *zip.Reader) {
//	self.zipFileReader = reader
//}

func (self *imageBuildBuilder) WithZipFilePath(path string) {
	self.zipFilePath = path
}

func (self *imageBuildBuilder) makeTarByZip(tarWriter *tar.Writer) (err error) {
	if self.zipFilePath == "" {
		return errors.New("Invalid zip path")
	}
	zipArchive, err := zip.OpenReader(self.zipFilePath)
	if err != nil {
		return err
	}
	defer zipArchive.Close()
	defer os.Remove(self.zipFilePath)

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
	return nil
}

func (self *imageBuildBuilder) Execute() (response types.ImageBuildResponse, err error) {
	tarArchive, err := os.CreateTemp("", "dpanel")
	slog.Info("tar path ", tarArchive.Name())
	if err != nil {
		return response, err
	}
	if self.imageBuildOption.RemoteContext != "" {
		response, err = Sdk.Client.ImageBuild(context.Background(), tarArchive, self.imageBuildOption)
		if err != nil {
			return response, err
		}
	} else {
		tarWriter := tar.NewWriter(tarArchive)
		defer tarWriter.Close()
		defer os.Remove(tarArchive.Name())

		if self.zipFilePath != "" {
			err = self.makeTarByZip(tarWriter)
			if err != nil {
				return response, err
			}
		}
		if self.dockerFileContent != nil {
			file, _ := os.CreateTemp("", "dpanel")
			defer file.Close()
			defer os.Remove(file.Name())

			file.Write(self.dockerFileContent)
			file.Seek(0, io.SeekStart)
			fileInfo, _ := file.Stat()
			fileInfoHeader, err := tar.FileInfoHeader(fileInfo, "")
			if err != nil {
				return response, err
			}
			fileInfoHeader.Name = "Dockerfile"
			err = tarWriter.WriteHeader(fileInfoHeader)
			if err != nil {
				return response, err
			}
			_, err = io.Copy(tarWriter, file)
			if err != nil {
				return response, err
			}
		}
		tarArchive.Seek(0, io.SeekStart)
		response, err = Sdk.Client.ImageBuild(context.Background(), tarArchive, self.imageBuildOption)
		if err != nil {
			return response, err
		}
	}
	return response, nil
}
