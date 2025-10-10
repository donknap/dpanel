package acme

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/exec/local"
)

const (
	DefaultCommandName     = "/root/.acme.sh/acme.sh"
	EnvOverrideCommandName = "ACME_OVERRIDE_COMMAND_NAME"
	EnvOverrideConfigHome  = "ACME_OVERRIDE_CONFIG_HOME"
)

func New(opts ...Option) (*Acme, error) {
	b := &Acme{
		commandName: DefaultCommandName,
		argv:        make([]string, 0),
	}
	if override := os.Getenv(EnvOverrideCommandName); override != "" {
		b.commandName = override
	}
	if override := os.Getenv(EnvOverrideConfigHome); override != "" {
		b.configHome = override
		b.argv = append(b.argv, "--config-home", override)
	} else {
		b.configHome = filepath.Dir(b.commandName)
	}
	b.argv = append(b.argv, "--ecc")
	for _, opt := range opts {
		err := opt(b)
		if err != nil {
			return nil, err
		}
	}
	return b, nil
}

type Acme struct {
	commandName string
	argv        []string
	env         []string
	configHome  string
}

func (self Acme) Run() (io.ReadCloser, error) {
	options := []local.Option{
		local.WithCommandName(self.commandName),
		local.WithArgs(self.argv...),
	}
	if !function.IsEmptyArray(self.env) {
		options = append(options, local.WithEnv(self.env))
	}
	cmd, err := local.New(options...)
	if err != nil {
		return nil, err
	}
	return cmd.RunInPip()
}

type Cert struct {
	RootPath      string   `json:"-"`
	MainDomain    string   `json:"mainDomain"`
	Domain        []string `json:"domain"`
	CA            string   `json:"CA"`
	CreatedAt     string   `json:"createdAt"`
	RenewAt       string   `json:"renewAt"`
	Success       bool     `json:"success"`
	DnsApi        string   `json:"dnsApi"`
	SslCrtContent string   `json:"sslCrtContent"`
	SslKeyContent string   `json:"sslKeyContent"`
}

func (self *Cert) IsImport() bool {
	return self.CA == "import"
}

func (self *Cert) FillCertContent() {
	if content, err := os.ReadFile(filepath.Join(self.GetRootPath(), "fullchain.cer")); err == nil {
		self.SslCrtContent = string(content)
	}
	if content, err := os.ReadFile(filepath.Join(self.GetRootPath(), self.Domain[0]+".key")); err == nil {
		self.SslKeyContent = string(content)
	}
}

func (self *Cert) GetRootPath() string {
	return self.RootPath + "_ecc"
}

func (self *Cert) GetConfigPath() string {
	return filepath.Join(self.GetRootPath(), self.Domain[0]+".conf")
}

func (self Acme) List() ([]*Cert, error) {
	self.argv = append(self.argv, "--list", "--listraw")
	cmd, err := local.New(
		local.WithCommandName(self.commandName),
		local.WithArgs(self.argv...),
	)
	if err != nil {
		return nil, err
	}
	out, err := cmd.RunWithResult()
	if err != nil {
		return nil, err
	}
	return self.ParseListRaw(out), nil
}

func (self Acme) ParseListRaw(out []byte) []*Cert {
	certList := make([]map[string]string, 0)
	certHeader := make([]string, 0)

	scanner := bufio.NewScanner(bytes.NewBuffer(out))
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "Main_Domain") {
			certHeader = strings.Split(scanner.Text(), "|")
			continue
		}
		if values := strings.Split(scanner.Text(), "|"); len(values) >= 6 {
			entry := make(map[string]string)
			for i, value := range values {
				if key := strings.TrimSpace(certHeader[i]); key != "" {
					entry[key] = value
				}
			}
			certList = append(certList, entry)
		}
	}

	result := make([]*Cert, 0)
	for _, item := range certList {
		domain := []string{
			item["Main_Domain"],
		}
		if item["SAN_Domains"] != "" && item["SAN_Domains"] != "no" {
			domain = append(domain, strings.Split(item["SAN_Domains"], ",")...)
		}

		success := false
		if item["CA"] != "" && item["Created"] != "" {
			success = true
		}
		cert := &Cert{
			MainDomain: item["Main_Domain"],
			RootPath:   filepath.Join(self.configHome, item["Main_Domain"]),
			Domain:     domain,
			CA:         item["CA"],
			CreatedAt:  item["Created"],
			RenewAt:    item["Renew"],
			Success:    success,
		}
		if !cert.IsImport() {
			if conf, err := os.ReadFile(cert.GetConfigPath()); err == nil {
				item := function.PluckArrayWalk(strings.Split(string(conf), "\n"), func(i string) (string, bool) {
					if k, v, exists := strings.Cut(i, "="); exists && k == "Le_Webroot" {
						return strings.Trim(v, "'"), true
					}
					return "", false
				})
				cert.DnsApi = strings.Join(item, "")
			}
		}
		result = append(result, cert)
	}
	return result
}

func (self Acme) Remove(name string) error {
	self.argv = append(self.argv, "--remove", "-d", name)
	cmd, err := local.New(
		local.WithCommandName(self.commandName),
		local.WithArgs(self.argv...),
	)
	if err != nil {
		return err
	}
	out, err := cmd.RunWithResult()
	if err != nil {
		return err
	}
	if strings.Contains(string(out), "has been removed") {
		return nil
	} else {
		return errors.New(string(out))
	}
}
