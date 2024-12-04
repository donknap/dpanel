package storage

import (
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"os"
	"path/filepath"
)

type Local struct {
}

func (self Local) Delete(name string) error {
	err := os.Remove(self.GetRealPath(name))
	return err
}

func (self Local) GetSaveRootPath() string {
	return filepath.Join(self.GetStorageLocalPath(), "storage")
}

func (self Local) GetRealPath(name string) string {
	return filepath.Join(self.GetStorageLocalPath(), "storage", name)
}

func (self Local) GetStorageCertPath() string {
	return filepath.Join(self.GetStorageLocalPath(), "cert")
}

func (self Local) GetComposePath() string {
	return filepath.Join(self.GetStorageLocalPath(), "compose")
}

func (self Local) GetStorePath() string {
	return filepath.Join(self.GetStorageLocalPath(), "store")
}

func (self Local) GetLicenseFilePath() string {
	return filepath.Join(self.GetStorageLocalPath(), "dpanel.lic")
}

func (self Local) GetStorageLocalPath() string {
	return facade.GetConfig().GetString("storage.local.path")
}
