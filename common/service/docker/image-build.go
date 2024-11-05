package docker

import (
	"archive/tar"
	"archive/zip"
	"errors"
	"github.com/docker/docker/api/types"
	"github.com/donknap/dpanel/common/function"
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
}

func (self *imageBuildBuilder) WithDockerFileContent(content []byte) {
	self.dockerFileContent = content
}

func (self *imageBuildBuilder) WithGitUrl(git string) {
	self.imageBuildOption.RemoteContext = git
}

func (self *imageBuildBuilder) WithDockerFilePath(path string) {
	self.imageBuildOption.Dockerfile = path
}

func (self *imageBuildBuilder) WithTag(name string) {
	self.imageBuildOption.Tags = append(self.imageBuildOption.Tags, name)
}

func (self *imageBuildBuilder) WithPlatform(name string, arch string) {
	self.imageBuildOption.Platform = name
	self.imageBuildOption.BuildArgs["TARGETARCH"] = function.PtrString(arch)
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
		return errors.New("invalid zip path")
	}
	zipArchive, err := zip.OpenReader(self.zipFilePath)
	if err != nil {
		return err
	}
	defer zipArchive.Close()

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
	os.Remove(self.zipFilePath)
	return nil
}

func (self *imageBuildBuilder) Execute() (response types.ImageBuildResponse, err error) {
	tarArchive, err := os.CreateTemp("", "dpanel")
	if err != nil {
		return response, err
	}

	if self.imageBuildOption.RemoteContext == "" {
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

			_, err = file.Write(self.dockerFileContent)
			if err != nil {
				slog.Error("docker", "image build write dockerfile", err.Error())
			}
			_, err = file.Seek(0, io.SeekStart)
			if err != nil {
				slog.Error("docker", "image build seek dockerfile", err.Error())
			}
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
	}
	response, err = Sdk.Client.ImageBuild(Sdk.Ctx, tarArchive, self.imageBuildOption)
	if err != nil {
		return response, err
	}
	return response, nil
}
