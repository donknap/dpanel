package imagefs

import (
	"errors"
	"os"

	"github.com/donknap/dpanel/common/service/fs/afs"
	filedata "github.com/donknap/dpanel/common/types/fs"
	"github.com/spf13/afero"
)

func (self *Fs) Users() (filedata.IdentityList, error) {
	return filedata.IdentityList{
		User:  []filedata.UserIdentity{},
		Group: []filedata.GroupIdentity{},
	}, nil
}

func (self *Fs) Open(string) (afero.File, error) {
	return nil, errors.ErrUnsupported
}

func (self *Fs) Stat(string) (os.FileInfo, error) {
	return nil, errors.ErrUnsupported
}

func (self *Fs) Info(string) (*filedata.FileData, error) {
	return nil, errors.ErrUnsupported
}

func (self *Fs) LstatIfPossible(string) (os.FileInfo, bool, error) {
	return nil, false, errors.ErrUnsupported
}

func (self *Fs) ReadlinkIfPossible(string) (string, error) {
	return "", errors.ErrUnsupported
}

func (self *Fs) ReadDir(string) ([]os.FileInfo, error) {
	return nil, errors.ErrUnsupported
}

func (self *Fs) PathSize(string) (int64, error) {
	return 0, errors.ErrUnsupported
}

func (self *Fs) Export([]afs.TransferFile) error {
	return errors.ErrUnsupported
}
