package docker

import (
	"archive/tar"
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/donknap/dpanel/common/function"
	"io"
	"os"
	"strings"
)

type imageBuildBuilder struct {
	//zipFileReader *zip.Reader
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

func (self *imageBuildBuilder) Execute() (err error) {
	os.Remove("/Users/renchao/Workspace/open-system/artifact-lskypro/test.tar")

	//tarArchive, err := os.CreateTemp("", "dpanel")
	tarArchive, err := os.Create("/Users/renchao/Workspace/open-system/artifact-lskypro/test.tar")
	if err != nil {
		return err
	}
	tarWriter := tar.NewWriter(tarArchive)
	defer tarWriter.Close()

	if self.zipFilePath != "" {
		err = self.makeTarByZip(tarWriter)
		if err != nil {
			return err
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
			return err
		}
		fileInfoHeader.Name = "Dockerfile"
		err = tarWriter.WriteHeader(fileInfoHeader)
		if err != nil {
			return err
		}
		_, err = io.Copy(tarWriter, file)
		if err != nil {
			return err
		}
	}
	tarArchive.Seek(0, io.SeekStart)
	response, err := self.dockerSdk.ImageBuild(context.Background(), tarArchive, types.ImageBuildOptions{
		Dockerfile: "Dockerfile",
		Tags: []string{
			"test" + function.GetMd5(function.GetRandomString(5)),
		},
	})
	if err != nil {
		fmt.Printf("%v \n", err)
		return nil
	}
	io.Copy(os.Stdout, response.Body)
	fmt.Printf("%v \n", err)
	return nil
}
