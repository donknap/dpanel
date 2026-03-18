package backup

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/mholt/archives"
)

type writer struct {
	file          *os.File
	tarPathPrefix string
	tarWriter     *tar.Writer
}

func (self writer) WriteBlob(content []byte) (path string, err error) {
	path, err = self.getBlobPath(function.Sha256(content))
	if err != nil {
		return path, err
	}
	if self.tarWriter == nil {
		return "", errors.New("context canceled")
	}
	buffer := bytes.NewBuffer(content)
	return self.WriteBlobReader(function.Sha256(content), io.NopCloser(buffer))
}

func (self writer) WriteBlobStruct(v interface{}) (path string, err error) {
	configContent, err := json.Marshal(v)
	if err != nil {
		return path, err
	}
	return self.WriteBlob(configContent)
}

func (self writer) WriteConfigFile(fileName string, v interface{}) error {
	content, err := json.Marshal(v)
	if err != nil {
		return err
	}
	err = self.tarWriter.WriteHeader(&tar.Header{
		Name:    fmt.Sprintf("%s/%s", self.tarPathPrefix, fileName),
		Size:    int64(len(content)),
		Mode:    0o644,
		ModTime: time.Now(),
	})
	if err != nil {
		return err
	}
	_, err = self.tarWriter.Write(content)
	if err != nil {
		return err
	}
	return nil
}

func (self writer) WriteBlobReader(sha256 string, reader io.ReadCloser) (path string, err error) {
	defer func() {
		if reader != nil {
			_ = reader.Close()
		}
	}()
	var tempFile *os.File
	path, err = self.getBlobPath(sha256)
	if err != nil {
		return path, err
	}
	tempFile, err = storage.Local{}.CreateTempFile(fmt.Sprintf("%s.%s.temp", filepath.Base(self.file.Name()), filepath.Base(path)))
	if err != nil {
		return path, err
	}
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
	}()

	gzWriter, err := archives.Gz{}.OpenWriter(tempFile)
	if err != nil {
		return path, err
	}
	_, err = io.Copy(gzWriter, reader)
	if err != nil {
		return path, err
	}
	err = gzWriter.Close()
	if err != nil {
		return path, err
	}
	_, _ = tempFile.Seek(io.SeekStart, 0)

	fileInfo, err := tempFile.Stat()
	if err != nil {
		return path, err
	}
	err = self.tarWriter.WriteHeader(&tar.Header{
		Name:    path,
		Size:    fileInfo.Size(),
		Mode:    int64(fileInfo.Mode()),
		ModTime: fileInfo.ModTime(),
	})

	if err != nil {
		return path, err
	}
	_, err = io.Copy(self.tarWriter, tempFile)
	if err != nil {
		return path, err
	}
	return strings.TrimLeft(path, self.tarPathPrefix), nil
}

func (self writer) WriteBlobFiles(sha256 string, files []archives.FileInfo) (path string, err error) {
	ctx := context.Background()

	var tempFile *os.File
	path, err = self.getBlobPath(sha256)
	if err != nil {
		return path, err
	}
	tempFile, err = storage.Local{}.CreateTempFile(fmt.Sprintf("%s.%s.temp", filepath.Base(self.file.Name()), filepath.Base(path)))
	if err != nil {
		return path, err
	}
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
	}()

	format := archives.CompressedArchive{
		Compression: archives.Gz{},
		Archival:    archives.Tar{},
	}
	err = format.Archive(ctx, tempFile, files)
	if err != nil {
		return path, err
	}
	_, _ = tempFile.Seek(io.SeekStart, 0)

	fileInfo, err := tempFile.Stat()
	if err != nil {
		return path, err
	}
	err = self.tarWriter.WriteHeader(&tar.Header{
		Name:    path,
		Size:    fileInfo.Size(),
		Mode:    int64(fileInfo.Mode()),
		ModTime: fileInfo.ModTime(),
	})
	if err != nil {
		return path, err
	}
	_, err = io.Copy(self.tarWriter, tempFile)
	if err != nil {
		return path, err
	}
	return strings.TrimLeft(path, self.tarPathPrefix), nil
}

func (self writer) getBlobPath(sha256 string) (p string, err error) {
	if b, a, ok := strings.Cut(sha256, ":"); ok {
		return path.Join(self.tarPathPrefix, "blobs", b, a), nil
	} else {
		return p, errors.New("invalid content sha256")
	}
}
