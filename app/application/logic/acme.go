package logic

import (
	"errors"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/exec"
	"github.com/donknap/dpanel/common/service/storage"
	"io"
	"strings"
)

type Acme struct {
}

const (
	commandName = "/root/.acme.sh/acme.sh"
)

type AcmeIssueOption struct {
	ServerName  []string
	CertServer  string
	Email       string
	AutoUpgrade bool
	Force       bool
	Renew       bool
	Debug       bool
	FileName    string
}

type acmeInfoResult struct {
	CreateTimeStr string
	RenewTimeStr  string
}

func (self AcmeIssueOption) to() ([]string, error) {
	var command []string
	if function.IsEmptyArray(self.ServerName) || self.Email == "" {
		return nil, errors.New("缺少生成参数")
	}
	if !function.IsEmptyArray(self.ServerName) {
		for _, serverName := range self.ServerName {
			command = append(command, "--domain", serverName)
		}
		settingPath := Site{}.GetSiteNginxSetting(self.ServerName[0])

		command = append(command, "--nginx")
		command = append(command, "--key-file", settingPath.KeyPath)
		command = append(command, "--fullchain-file", settingPath.CertPath)
	}
	if self.CertServer != "" {
		command = append(command, "--server", self.CertServer)
	}
	if self.Email != "" {
		command = append(command, "--email", self.Email)
	}
	if self.AutoUpgrade {
		command = append(command, "--auto-upgrade", "1")
	}
	if self.Force {
		command = append(command, "--force")
	}
	if self.Debug {
		command = append(command, "--debug")
	}
	command = append(command, "--config-home", storage.Local{}.GetStorageLocalPath()+"/acme")
	return command, nil
}

func (self Acme) Issue(option *AcmeIssueOption) (io.ReadCloser, error) {
	command, err := option.to()
	if err != nil {
		return nil, err
	}
	if option.Renew {
		command = append(command, "--renew", "--force")
	} else {
		command = append(command, "--issue")
	}
	out, err := exec.Command{}.RunInTerminal(&exec.RunCommandOption{
		CmdName: commandName,
		CmdArgs: command,
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (self Acme) Info(serverName string) *acmeInfoResult {
	out := exec.Command{}.RunWithResult(&exec.RunCommandOption{
		CmdName: commandName,
		CmdArgs: []string{
			"--config-home", storage.Local{}.GetStorageLocalPath() + "/acme",
			"--info",
			"--domain", serverName,
		},
	})
	result := &acmeInfoResult{}
	for _, row := range strings.Split(out, "\n") {
		if strings.HasPrefix(row, "Le_CertCreateTimeStr=") {
			value, _ := strings.CutPrefix(row, "Le_CertCreateTimeStr=")
			result.CreateTimeStr = value
		}
		if strings.HasPrefix(row, "Le_NextRenewTimeStr=") {
			value, _ := strings.CutPrefix(row, "Le_NextRenewTimeStr=")
			result.RenewTimeStr = value
		}
	}
	return result
}
