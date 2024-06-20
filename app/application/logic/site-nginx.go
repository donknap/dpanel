package logic

import (
	"fmt"
	"github.com/we7coreteam/w7-rangine-go-support/src/facade"
	"os"
)

type siteNginxSettingResult struct {
	CertPath string
	KeyPath  string
	ConfPath string
}

func (self siteNginxSettingResult) GetCertContent() ([]byte, error) {
	return os.ReadFile(self.CertPath)
}

func (self siteNginxSettingResult) GetKeyContent() ([]byte, error) {
	return os.ReadFile(self.KeyPath)
}

func (self siteNginxSettingResult) GetConfContent() ([]byte, error) {
	return os.ReadFile(self.ConfPath)
}

func (self siteNginxSettingResult) RemoveAll() {
	os.Remove(self.ConfPath)
	os.Remove(self.KeyPath)
	os.Remove(self.CertPath)
}
func (self Site) GetSiteNginxSetting(serverName string) *siteNginxSettingResult {
	return &siteNginxSettingResult{
		CertPath: self.getNginxCertPath() + fmt.Sprintf(CertFileName, serverName),
		KeyPath:  self.getNginxCertPath() + fmt.Sprintf(KeyFileName, serverName),
		ConfPath: self.getNginxSettingPath() + fmt.Sprintf(VhostFileName, serverName),
	}
}

func (self Site) getNginxSettingPath() string {
	return fmt.Sprintf("%s/nginx/proxy_host/", facade.GetConfig().Get("storage.local.path"))
}

func (self Site) getNginxCertPath() string {
	return fmt.Sprintf("%s/cert/", facade.GetConfig().Get("storage.local.path"))
}
