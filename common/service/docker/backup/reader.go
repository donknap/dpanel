package backup

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"github.com/donknap/dpanel/common/function"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type blobItem struct {
	Name   string
	Offset int64
}

type reader struct {
	file          *os.File
	tarPathPrefix string
	blobs         []blobItem
}

func (self *reader) Info() (*Info, error) {
	tarReader := tar.NewReader(self.file)
	info := &Info{}
	for {
		header, err := tarReader.Next()
		if err != nil {
			break
		}
		headerName := strings.TrimLeft(header.Name, "/")
		if strings.HasSuffix(headerName, "info.json") {
			content, err := io.ReadAll(tarReader)
			if err != nil {
				return nil, err
			}
			err = json.Unmarshal(content, &info)
			if err != nil {
				return nil, err
			}
			return info, nil
		}
	}
	return nil, errors.New("info file not found in archive")
}

func (self *reader) Manifest() ([]Manifest, error) {
	var offset int64
	tarReader := tar.NewReader(self.file)
	m := make([]Manifest, 0)
	for {
		header, err := tarReader.Next()
		if err != nil {
			break
		}
		headerName := strings.TrimLeft(header.Name, "/")
		if headerName == filepath.Join(self.tarPathPrefix, "manifest.json") {
			content, err := io.ReadAll(tarReader)
			if err != nil {
				return nil, err
			}
			err = json.Unmarshal(content, &m)
			if err != nil {
				return nil, err
			}
		}
		if strings.HasPrefix(headerName, filepath.Join(self.tarPathPrefix, "blobs/sha256/")) {
			offset, _ = self.file.Seek(0, io.SeekCurrent)
			self.blobs = append(self.blobs, blobItem{
				Name:   headerName,
				Offset: offset - 512,
			})
		}
	}
	if function.IsEmptyArray(m) {
		return nil, errors.New("manifest file not found in archive")
	}
	return m, nil
}

func (self *reader) ReadBlobs(fileName string) (io.Reader, error) {
	var index int
	var ok bool
	if ok, index = function.IndexArrayWalk(self.blobs, func(i blobItem) bool {
		return i.Name == filepath.Join(self.tarPathPrefix, fileName)
	}); !ok {
		return nil, errors.New("blob file not found in archive")
	}
	_, err := self.file.Seek(self.blobs[index].Offset, io.SeekStart)
	if err != nil {
		return nil, err
	}
	tarReader := tar.NewReader(self.file)
	_, err = tarReader.Next()
	if err != nil {
		return nil, err
	}
	return tarReader, nil
}

func (self *reader) ReadBlobsContent(fileName string) ([]byte, error) {
	out, err := self.ReadBlobs(fileName)
	if err != nil {
		return nil, err
	}
	gzReader, err := gzip.NewReader(out)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = gzReader.Close()
	}()
	return io.ReadAll(gzReader)
}
