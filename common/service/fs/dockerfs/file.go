package dockerfs

import (
	"errors"
	"github.com/spf13/afero/mem"
	"os"
)

type File struct {
	info os.FileInfo
	fd   *mem.File
	fs   *Fs
}

func (self *File) Close() error {
	if self.fd == nil {
		return nil
	}
	return self.fd.Close()
}

func (self *File) Read(p []byte) (n int, err error) {
	if self.fd == nil {
		return 0, &os.PathError{Op: "close", Path: self.info.Name(), Err: nil}
	}
	return self.fd.Read(p)
}

func (self *File) ReadAt(p []byte, off int64) (n int, err error) {
	return self.fd.ReadAt(p, off)
}

func (self *File) Seek(offset int64, whence int) (int64, error) {
	return self.fd.Seek(offset, whence)
}

func (self *File) Write(p []byte) (n int, err error) {
	return self.fd.Write(p)
}

func (self *File) WriteAt(p []byte, off int64) (n int, err error) {
	return self.fd.WriteAt(p, off)
}

func (self *File) Name() string {
	return self.fd.Name()
}

func (self *File) Readdir(count int) ([]os.FileInfo, error) {
	if self.info.Mode().IsRegular() {
		return nil, errors.New("targe is not a directory")
	}
	return self.fs.readDirFromContainer(self.info.Name())
}

func (self *File) Readdirnames(n int) ([]string, error) {
	return self.fd.Readdirnames(n)
}

func (self *File) Stat() (os.FileInfo, error) {
	return self.fd.Stat()
}

func (self *File) Sync() error {
	return self.fd.Sync()
}

func (self *File) Truncate(size int64) error {
	return self.fd.Truncate(size)
}

func (self *File) WriteString(s string) (ret int, err error) {
	return self.fd.WriteString(s)
}
