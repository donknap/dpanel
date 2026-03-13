package imports

import (
	"archive/tar"
	"io"
	"os"

	"github.com/donknap/dpanel/common/service/storage"
)

type ImportFile struct {
	tempFilePath   string // 导入存放的临时 tar 文件
	targetRootPath string
	tarWrite       *tar.Writer
	tarReader      io.ReadCloser
	io.Closer
}

func (self ImportFile) Reader() io.Reader {
	f, _ := os.Open(self.tempFilePath)
	return f
}

func (self ImportFile) TarReader() *tar.Reader {
	return tar.NewReader(self.Reader())
}

func (self ImportFile) Close() {
	if self.tarReader != nil {
		_ = self.tarReader.Close()
	}
	if self.tarWrite != nil {
		_ = self.tarWrite.Close()
	}
	_ = os.Remove(self.tempFilePath)
}

type ImportFileOption func(self *ImportFile) (err error)

func NewFileImport(targetRootPath string, opts ...ImportFileOption) (*ImportFile, error) {
	var err error
	o := &ImportFile{
		targetRootPath: targetRootPath,
	}
	tempFile, err := storage.Local{}.CreateTempFile("")
	if err != nil {
		return nil, err
	}
	o.tempFilePath = tempFile.Name()
	defer func() {
		_ = tempFile.Close()
	}()

	o.tarWrite = tar.NewWriter(tempFile)
	for _, opt := range opts {
		if err := opt(o); err != nil {
			o.Close()
			return nil, err
		}
	}
	if err := o.tarWrite.Close(); err != nil {
		o.Close()
		return nil, err
	}
	return o, nil
}
