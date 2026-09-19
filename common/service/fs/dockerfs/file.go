package dockerfs

import (
	"errors"
	"io"
	"os"

	"github.com/donknap/dpanel/common/service/fs/tempfs"
	fsdata "github.com/donknap/dpanel/common/types/fs"
)

type File struct {
	fs          *Fs
	name        string
	backendName string
	fd          *os.File
	info        os.FileInfo
	entries     []os.FileInfo
	directory   bool
	readable    bool
	writable    bool
	syncMode    bool
	loadContent bool
	dirty       bool
	closed      bool
	dirOffset   int
	temporaryFs *tempfs.Fs
}

func (self *File) Close() error {
	if self.closed {
		return &os.PathError{Op: "close", Path: self.name, Err: os.ErrInvalid}
	}
	if self.directory {
		self.closed = true
		return nil
	}
	syncErr := self.Sync()
	self.closed = true
	var closeErr error
	var cleanupErr error
	if self.fd != nil {
		closeErr = self.fd.Close()
		cleanupErr = self.temporaryFs.Close()
	}
	return errors.Join(syncErr, closeErr, cleanupErr)
}

func (self *File) Read(p []byte) (int, error) {
	if err := self.checkRegular("read", self.readable); err != nil {
		return 0, err
	}
	if err := self.materialize(); err != nil {
		return 0, err
	}
	return self.fd.Read(p)
}

func (self *File) ReadAt(p []byte, off int64) (int, error) {
	if err := self.checkRegular("readat", self.readable); err != nil {
		return 0, err
	}
	if off < 0 {
		return 0, &os.PathError{Op: "readat", Path: self.name, Err: os.ErrInvalid}
	}
	if err := self.materialize(); err != nil {
		return 0, err
	}
	return self.fd.ReadAt(p, off)
}

func (self *File) Seek(offset int64, whence int) (int64, error) {
	if err := self.checkRegular("seek", true); err != nil {
		return 0, err
	}
	if err := self.materialize(); err != nil {
		return 0, err
	}
	var base int64
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		position, err := self.fd.Seek(0, io.SeekCurrent)
		if err != nil {
			return 0, err
		}
		base = position
	case io.SeekEnd:
		info, err := self.fd.Stat()
		if err != nil {
			return 0, err
		}
		base = info.Size()
	default:
		return 0, &os.PathError{Op: "seek", Path: self.name, Err: os.ErrInvalid}
	}
	position := base + offset
	if position < 0 {
		return 0, &os.PathError{Op: "seek", Path: self.name, Err: os.ErrInvalid}
	}
	return self.fd.Seek(position, io.SeekStart)
}

func (self *File) Write(p []byte) (int, error) {
	if err := self.checkRegular("write", self.writable); err != nil {
		return 0, err
	}
	if err := self.materialize(); err != nil {
		return 0, err
	}
	n, err := self.fd.Write(p)
	if n > 0 {
		self.dirty = true
		err = errors.Join(err, self.refreshMetadata())
	}
	if err == nil && self.syncMode {
		err = self.Sync()
	}
	return n, err
}

func (self *File) WriteAt(p []byte, off int64) (int, error) {
	if err := self.checkRegular("writeat", self.writable); err != nil {
		return 0, err
	}
	if off < 0 {
		return 0, &os.PathError{Op: "writeat", Path: self.name, Err: os.ErrInvalid}
	}
	if err := self.materialize(); err != nil {
		return 0, err
	}
	position, err := self.fd.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	n, writeErr := self.fd.WriteAt(p, off)
	_, seekErr := self.fd.Seek(position, io.SeekStart)
	if n > 0 {
		self.dirty = true
		writeErr = errors.Join(writeErr, self.refreshMetadata())
	}
	err = errors.Join(writeErr, seekErr)
	if err == nil && self.syncMode {
		err = self.Sync()
	}
	return n, err
}

func (self *File) Name() string {
	return self.name
}

func (self *File) Readdir(count int) ([]os.FileInfo, error) {
	if self.closed {
		return nil, &os.PathError{Op: "readdir", Path: self.name, Err: os.ErrInvalid}
	}
	if !self.directory {
		return nil, &os.PathError{Op: "readdir", Path: self.name, Err: errors.New("not a directory")}
	}
	if self.entries == nil {
		entries, err := self.fs.readDirFromContainer(self.name)
		if err != nil {
			return nil, err
		}
		self.entries = entries
	}
	if self.dirOffset >= len(self.entries) {
		if count > 0 {
			return []os.FileInfo{}, io.EOF
		}
		return []os.FileInfo{}, nil
	}
	end := len(self.entries)
	if count > 0 && self.dirOffset+count < end {
		end = self.dirOffset + count
	}
	result := self.entries[self.dirOffset:end]
	self.dirOffset = end
	return result, nil
}

func (self *File) Readdirnames(count int) ([]string, error) {
	entries, err := self.Readdir(count)
	names := make([]string, len(entries))
	for index, entry := range entries {
		names[index] = entry.Name()
	}
	return names, err
}

func (self *File) Stat() (os.FileInfo, error) {
	if self.closed {
		return nil, &os.PathError{Op: "stat", Path: self.name, Err: os.ErrInvalid}
	}
	return self.info, nil
}

func (self *File) Sync() error {
	if self.closed {
		return &os.PathError{Op: "sync", Path: self.name, Err: os.ErrInvalid}
	}
	if self.directory || !self.dirty {
		return nil
	}
	if err := self.materialize(); err != nil {
		return err
	}
	if err := self.fs.writeFile(self.backendName, self.fd, self.info); err != nil {
		return err
	}
	self.dirty = false
	return nil
}

func (self *File) Truncate(size int64) error {
	if err := self.checkRegular("truncate", self.writable); err != nil {
		return err
	}
	if self.fd == nil && size == 0 {
		self.loadContent = false
	}
	if err := self.materialize(); err != nil {
		return err
	}
	if err := self.fd.Truncate(size); err != nil {
		return err
	}
	self.dirty = true
	if err := self.refreshMetadata(); err != nil {
		return err
	}
	if self.syncMode {
		return self.Sync()
	}
	return nil
}

func (self *File) refreshMetadata() error {
	if self.fd == nil {
		return nil
	}
	info, err := self.fd.Stat()
	if err != nil {
		return err
	}
	if data, ok := self.info.Sys().(*fsdata.FileData); ok {
		data.Size = info.Size()
		data.ModTime = info.ModTime()
	}
	return nil
}

func (self *File) materialize() (err error) {
	if self.fd != nil {
		return nil
	}
	temporaryFileSystem, err := tempfs.New()
	if err != nil {
		return err
	}
	file, err := temporaryFileSystem.Create("/file")
	if err != nil {
		return errors.Join(err, temporaryFileSystem.Close())
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, file.Close(), temporaryFileSystem.Close())
		}
	}()
	if self.loadContent {
		if err = self.fs.readFile(self.backendName, file); err != nil {
			return err
		}
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err = file.Chmod(0o600); err != nil {
		return err
	}
	self.fd = file
	self.temporaryFs = temporaryFileSystem
	self.loadContent = false
	return nil
}

func (self *File) WriteString(value string) (int, error) {
	return self.Write([]byte(value))
}

func (self *File) checkRegular(operation string, allowed bool) error {
	if self.closed {
		return &os.PathError{Op: operation, Path: self.name, Err: os.ErrInvalid}
	}
	if self.directory {
		return &os.PathError{Op: operation, Path: self.name, Err: errors.New("is a directory")}
	}
	if !allowed {
		return &os.PathError{Op: operation, Path: self.name, Err: os.ErrPermission}
	}
	return nil
}
