package storage

import (
	"fmt"
	"github.com/donknap/dpanel/common/service/acme"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"log/slog"
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

func (self Local) GetScriptTemplatePath() string {
	return filepath.Join(self.GetStorageLocalPath(), "script")
}

func (self Local) GetBackupPath() string {
	return filepath.Join(self.GetStorageLocalPath(), "backup")
}

func (self Local) GetStorageLocalPath() string {
	if facade.GetConfig() == nil {
		slog.Debug("storage local path empty")
		return ""
	}
	return facade.GetConfig().GetString("storage.local.path")
}

func (self Local) GetNginxSettingPath() string {
	return fmt.Sprintf("%s/nginx/proxy_host/", self.GetStorageLocalPath())
}

func (self Local) GetNginxCertPath() string {
	if override := os.Getenv(acme.EnvOverrideConfigHome); override != "" {
		return override
	}
	return fmt.Sprintf("%s/cert/", self.GetStorageLocalPath())
}
