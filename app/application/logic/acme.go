package logic

import (
	"errors"
	"github.com/donknap/dpanel/common/service/exec"
	"strings"
)

type Acme struct {
}

const (
	commandName = "/root/.acme.sh/acme.sh"
)

type AcmeIssueOption struct {
	ServerName  string
	CertServer  string
	Email       string
	AutoUpgrade bool
	Force       bool
	Renew       bool
	Debug       bool
}

type acmeInfoResult struct {
	CreateTimeStr string
	RenewTimeStr  string
}

func (self AcmeIssueOption) to() ([]string, error) {
	var command []string
	if self.ServerName == "" || self.Email == "" {
		return nil, errors.New("缺少生成参数")
	}
	if self.ServerName != "" {
		command = append(command, "--domain", self.ServerName)

		settingPath := Site{}.GetSiteNginxSetting(self.ServerName)
		command = append(command, "--nginx", settingPath.ConfPath)
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

	return command, nil
}

func (self Acme) Issue(option *AcmeIssueOption) error {
	command, err := option.to()
	if err != nil {
		return err
	}
	if option.Renew {
		command = append(command, "--renew", "--force")
	} else {
		command = append(command, "--issue")
	}

	exec.Command{}.Run(&exec.RunCommandOption{
		CmdName: commandName,
		CmdArgs: command,
	})
	return nil
}

func (self Acme) Info(serverName string) *acmeInfoResult {
	out := exec.Command{}.RunWithOut(&exec.RunCommandOption{
		CmdName: commandName,
		CmdArgs: []string{
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
