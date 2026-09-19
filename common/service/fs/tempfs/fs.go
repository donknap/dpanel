package tempfs

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/donknap/dpanel/common/service/storage"
)

type Fs struct {
	rootPath string
	root     *os.Root
}

// TODO: Migrate project-wide temporary file and directory handling to tempfs.
func New() (*Fs, error) {
	rootPath, err := (storage.Local{}).CreateTempDir("")
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, errors.Join(err, os.RemoveAll(rootPath))
	}
	return &Fs{rootPath: rootPath, root: root}, nil
}

func (self *Fs) Close() error {
	if self == nil || self.root == nil {
		return nil
	}
	root := self.root
	self.root = nil
	return errors.Join(root.Close(), os.RemoveAll(self.rootPath))
}

func (self *Fs) LocalPath(name string) (string, error) {
	localName, err := self.localName(name)
	if err != nil {
		return "", err
	}
	return filepath.Join(self.rootPath, localName), nil
}

func (self *Fs) Create(name string) (*os.File, error) {
	localName, err := self.localName(name)
	if err != nil {
		return nil, &os.PathError{Op: "create", Path: name, Err: err}
	}
	return self.root.Create(localName)
}

func (self *Fs) Open(name string) (*os.File, error) {
	localName, err := self.localName(name)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: name, Err: err}
	}
	return self.root.Open(localName)
}

func (self *Fs) MkdirAll(name string, perm os.FileMode) error {
	localName, err := self.localName(name)
	if err != nil {
		return &os.PathError{Op: "mkdir", Path: name, Err: err}
	}
	return self.root.MkdirAll(localName, perm)
}

func (self *Fs) ReadDir(name string) ([]os.DirEntry, error) {
	localName, err := self.localName(name)
	if err != nil {
		return nil, &os.PathError{Op: "readdir", Path: name, Err: err}
	}
	directory, err := self.root.Open(localName)
	if err != nil {
		return nil, err
	}
	entries, readErr := directory.ReadDir(-1)
	return entries, errors.Join(readErr, directory.Close())
}

func (self *Fs) localName(name string) (string, error) {
	if self == nil || self.root == nil {
		return "", os.ErrClosed
	}
	if name == "" || strings.IndexByte(name, 0) >= 0 || !path.IsAbs(name) || path.Clean(name) != name {
		return "", errors.New("invalid virtual absolute path")
	}
	name = strings.TrimPrefix(name, "/")
	if name == "" {
		return ".", nil
	}
	return filepath.FromSlash(name), nil
}
