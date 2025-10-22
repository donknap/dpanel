package imports

import (
	"archive/tar"
	"io"
	"os"

	"github.com/donknap/dpanel/common/service/storage"
)

type ImportFile struct {
	targetRootPath string
	tarWrite       *tar.Writer
	reader         *os.File
	io.Closer
}

func (self ImportFile) Reader() io.Reader {
	_, _ = self.reader.Seek(0, io.SeekStart)
	return self.reader
}

func (self ImportFile) TarReader() *tar.Reader {
	_, _ = self.reader.Seek(0, io.SeekStart)
	return tar.NewReader(self.reader)
}

func (self ImportFile) Close() {
	_ = self.reader.Close()
	_ = os.Remove(self.reader.Name())
}

type ImportFileOption func(self *ImportFile) (err error)

func NewFileImport(targetRootPath string, opts ...ImportFileOption) (*ImportFile, error) {
	var err error
	o := &ImportFile{
		targetRootPath: targetRootPath,
	}
	o.reader, err = storage.Local{}.CreateTempFile("")
	if err != nil {
		return nil, err
	}
	o.tarWrite = tar.NewWriter(o.reader)
	for _, opt := range opts {
		err := opt(o)
		if err != nil {
			return nil, err
		}
	}
	return o, nil
}
