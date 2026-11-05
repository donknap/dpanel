package afs

import (
	"os"

	"github.com/donknap/dpanel/common/types/fs"
	"github.com/spf13/afero"
)

type TransferFile struct {
	Source string
	Target string
}

type ReadFs interface {
	afero.LinkReader
	afero.Lstater
	Name() string
	Open(string) (afero.File, error)
	Stat(string) (os.FileInfo, error)
	ReadDir(string) ([]os.FileInfo, error)
	List(string) ([]*fs.FileData, error)
	Info(string) (*fs.FileData, error)
	WorkingDir() string
	RootDirs() ([]string, error)
	Destroy() error
	PathSize(string) (int64, error)
	Users() (fs.IdentityList, error)
	Export([]TransferFile) error
}

type Fs interface {
	ReadFs
	afero.Fs
	RemoveAll(string) error
	Copy(source, target string, overwrite bool) error
	Move(source, target string, overwrite bool) error
	ChmodAll(string, os.FileMode, bool) error
	ChownAll(string, *int, *int, bool) error
	Import([]TransferFile) error
	UnArchive([]string, string) error
}
