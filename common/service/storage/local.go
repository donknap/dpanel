package storage

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/donknap/dpanel/common/service/acme"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/google/uuid"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

type Local struct {
}

func (self Local) Delete(name string) error {
	err := os.Remove(self.GetSaveRealPath(name))
	return err
}

func (self Local) GetSaveRootPath() string {
	return filepath.Join(self.GetStorageLocalPath(), "storage")
}

func (self Local) GetSaveRealPath(name string) string {
	return filepath.Join(self.GetStorageLocalPath(), "storage", name)
}

func (self Local) GetCertPath() string {
	return filepath.Join(self.GetStorageLocalPath(), "cert")
}

func (self Local) GetCertDomainPath() string {
	if override := os.Getenv(acme.EnvOverrideConfigHome); override != "" {
		return override
	}
	return fmt.Sprintf("%s/acme/", self.GetStorageLocalPath())
}

func (self Local) GetComposePath(prefix string) string {
	if prefix == "" || prefix == define.DockerDefaultClientName {
		return filepath.Join(self.GetStorageLocalPath(), "compose")
	} else {
		return filepath.Join(self.GetStorageLocalPath(), "compose-"+prefix)
	}
}

func (self Local) GetStorePath() string {
	return filepath.Join(self.GetStorageLocalPath(), "store")
}

func (self Local) GetLicenseFilePath() string {
	return filepath.Join(self.GetStorageLocalPath(), "dpanel.lic")
}

func (self Local) GetW7OpenSoftwareLicenseFilePath() string {
	return filepath.Join(self.GetStorageLocalPath(), "w7_opensoftware.enc")
}

func (self Local) GetScriptTemplatePath() string {
	return filepath.Join(self.GetStorageLocalPath(), "script")
}

func (self Local) GetBackupPath() string {
	return filepath.Join(self.GetStorageLocalPath(), "backup")
}

func (self Local) GetLocalProxySockPath() string {
	path := filepath.Join(self.GetStorageLocalPath(), "sock")
	return path
}

func (self Local) GetNginxSettingPath() string {
	return fmt.Sprintf("%s/nginx/proxy_host/", self.GetStorageLocalPath())
}

func (self Local) GetNginxExtraSettingPath() string {
	return fmt.Sprintf("%s/nginx/extra_host/", self.GetStorageLocalPath())
}

func (self Local) GetStorageLocalPath() string {
	if facade.GetConfig() == nil {
		slog.Debug("storage local path empty")
		return ""
	}
	if v := facade.GetConfig().GetString("system.storage.local.path"); v == "" {
		panic("invalid local storage path")
	} else {
		return v
	}
}

func (self Local) CreateSaveFile(name string) (*os.File, error) {
	f := filepath.Join(self.GetSaveRootPath(), name)
	_ = os.MkdirAll(filepath.Dir(f), os.ModePerm)
	return os.Create(f)
}

func (self Local) GetLocalTempDir() string {
	p := filepath.Join(self.GetSaveRootPath(), "temp")
	if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
		_ = os.MkdirAll(p, os.ModePerm)
	}
	return p
}

func (self Local) GetTempFile(name string) (file *os.File, path string, err error) {
	tempFilePath := filepath.Join(self.GetLocalTempDir(), name)
	_, err = os.Stat(tempFilePath)
	if err != nil {
		return nil, "", err
	}
	file, err = os.OpenFile(tempFilePath, os.O_RDWR, os.ModePerm)
	if err != nil {
		return nil, "", err
	}
	return file, file.Name(), err
}

func (self Local) CreateTempFile(name string) (*os.File, error) {
	if name == "" {
		return os.CreateTemp(self.GetLocalTempDir(), "dpanel-temp-")
	}
	return os.Create(filepath.Join(self.GetLocalTempDir(), name))
}

func (self Local) CreateTempDir(name string) (string, error) {
	if name == "" {
		return os.MkdirTemp(self.GetLocalTempDir(), "dpanel-temp-")
	}
	name = fmt.Sprintf("dpanel-temp-%s", name)
	path := filepath.Join(self.GetLocalTempDir(), name)
	if _, err := os.Stat(path); err == nil {
		_ = os.RemoveAll(path)
	}
	err := os.MkdirAll(path, os.ModePerm)
	return path, err
}

func (self Local) SaveUploadImage(uploadFileName, newFileNamePrefix string, appendRandomString bool) string {
	// 删除旧的前缀文件
	rootPath := filepath.Join(self.GetSaveRootPath(), "image")
	if matches, err := filepath.Glob(filepath.Join(rootPath, newFileNamePrefix+"*")); err == nil {
		for _, match := range matches {
			_ = os.Remove(match)
		}
	}
	var newFileName string
	if appendRandomString {
		newFileName = fmt.Sprintf("%s-%s.png", newFileNamePrefix, uuid.New().String()[24:])
	} else {
		newFileName = fmt.Sprintf("%s.png", newFileNamePrefix)
	}

	newBgFile := filepath.Join(rootPath, newFileName)
	_ = os.MkdirAll(filepath.Dir(newBgFile), 0777)
	err := os.Rename(
		uploadFileName,
		newBgFile,
	)
	if err != nil {
		slog.Debug("save upload image fail", "error", err)
	}
	return "/dpanel/static/image/" + newFileName
}
