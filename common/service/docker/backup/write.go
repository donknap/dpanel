package backup

import (
	"archive/tar"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/donknap/dpanel/common/function"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type write struct {
	file          *os.File
	tarPathPrefix string
	tarWriter     *tar.Writer
}

func (self write) WriteBlob(content []byte) (path string, err error) {
	path, err = self.getBlobPath(function.GetSha256(content))
	if err != nil {
		return path, err
	}
	err = self.tarWriter.WriteHeader(&tar.Header{
		Name:    path,
		Size:    int64(len(content)),
		Mode:    0o644,
		ModTime: time.Now(),
	})
	if err != nil {
		return path, err
	}
	_, err = self.tarWriter.Write(content)
	if err != nil {
		return path, err
	}
	return strings.TrimLeft(path, self.tarPathPrefix), nil
}

func (self write) WriteBlobStruct(v interface{}) (path string, err error) {
	configContent, err := json.Marshal(v)
	if err != nil {
		return path, err
	}
	return self.WriteBlob(configContent)
}

func (self write) WriteManifest(v interface{}) error {
	content, err := json.Marshal(v)
	if err != nil {
		return err
	}
	err = self.tarWriter.WriteHeader(&tar.Header{
		Name:    fmt.Sprintf("%s/manifest.json", self.tarPathPrefix),
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

func (self write) WriteBlobReader(sha256 string, out io.ReadCloser) (path string, err error) {
	var tempFile *os.File
	path, err = self.getBlobPath(sha256)
	if err != nil {
		return path, err
	}
	tempFile, err = os.OpenFile(fmt.Sprintf("%s.%s.temp", self.file.Name(), filepath.Base(path)), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return path, err
	}
	defer func() {
		_ = tempFile.Close()
		//_ = os.Remove(tempFile.Name())
	}()
	defer func() {
		_ = out.Close()
	}()
	_, err = io.Copy(tempFile, out)
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

func (self write) getBlobPath(sha256 string) (path string, err error) {
	if b, a, ok := strings.Cut(sha256, ":"); ok {
		return filepath.Join(self.tarPathPrefix, "blobs", b, a), nil
	} else {
		return path, errors.New("invalid content sha256")
	}
}
