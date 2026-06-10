package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/donknap/dpanel/common/function"
	"github.com/mholt/archives"
)

type blobItem struct {
	Name   string
	Offset int64
}

type reader struct {
	file  *os.File
	blobs []blobItem
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
	_, err := self.file.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
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
		if strings.HasSuffix(header.Name, "manifest.json") {
			content, err := io.ReadAll(tarReader)
			if err != nil {
				return nil, err
			}
			err = json.Unmarshal(content, &m)
			if err != nil {
				return nil, err
			}
		}
		if strings.Contains(header.Name, "blobs/sha256/") {
			offset, _ = self.file.Seek(0, io.SeekCurrent)
			self.blobs = append(self.blobs, blobItem{
				Name:   header.Name,
				Offset: offset - 512,
			})
		}
	}
	if function.IsEmptyArray(m) {
		return nil, errors.New("manifest file not found in archive")
	}
	_, err := self.file.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (self *reader) ReadBlobs(fileName string) (io.Reader, error) {
	var index int
	var ok bool
	if index, ok = function.IndexArrayWalk(self.blobs, func(i blobItem) bool {
		return strings.HasSuffix(i.Name, fileName)
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

func (self *reader) Extract(fileName string, targetPath string) error {
	out, err := self.ReadBlobs(fileName)
	if err != nil {
		return err
	}
	targetAbs, err := filepath.Abs(filepath.Clean(targetPath))
	if err != nil {
		return err
	}

	ctx := context.Background()
	format, stream, err := archives.Identify(ctx, fileName, out)
	if err != nil {
		return err
	}
	ex, ok := format.(archives.Extractor)
	if !ok {
		return err
	}
	handler := func(ctx context.Context, f archives.FileInfo) error {
		if f.NameInArchive == "" || f.NameInArchive == "." {
			return errors.New("invalid archive path")
		}
		outPath := function.SafePathJoin(targetAbs, f.NameInArchive)
		outAbs, err := filepath.Abs(filepath.Clean(outPath))
		if err != nil {
			return err
		}
		if outAbs == targetAbs {
			return errors.New("invalid archive path")
		}
		if !f.IsDir() && !f.Mode().IsRegular() {
			return nil
		}
		if f.IsDir() {
			return os.MkdirAll(outAbs, f.Mode())
		}
		if err := os.MkdirAll(filepath.Dir(outAbs), 0755); err != nil {
			return err
		}
		out, err := os.OpenFile(outAbs, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		defer out.Close()
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		_, err = io.Copy(out, rc)
		return err
	}
	err = ex.Extract(ctx, stream, handler)
	if err != nil {
		return err
	}
	return nil
}
