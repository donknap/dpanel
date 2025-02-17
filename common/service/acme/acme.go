package acme

import (
	"bufio"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/exec"
	"io"
	"os"
	"path/filepath"
	"strings"
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
	options := []exec.Option{
		exec.WithCommandName(self.commandName),
		exec.WithArgs(self.argv...),
	}
	if !function.IsEmptyArray(self.env) {
		options = append(options, exec.WithEnv(self.env))
	}
	cmd, err := exec.New(options...)
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
	if self.IsImport() {
		return self.RootPath
	} else {
		return self.RootPath + "_ecc"
	}
}

func (self *Cert) GetConfigPath() string {
	return filepath.Join(self.GetRootPath(), self.Domain[0]+".conf")
}

func (self Acme) List() ([]*Cert, error) {
	cmd, err := exec.New(
		exec.WithCommandName(self.commandName),
		exec.WithArgs("--list", "--listraw"),
	)
	if err != nil {
		return nil, err
	}
	out, err := cmd.Run()
	if err != nil {
		return nil, err
	}
	result := make([]*Cert, 0)
	scanner := bufio.NewScanner(out)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "Main_Domain") {
			continue
		}
		if split := strings.Split(scanner.Text(), "|"); len(split) >= 6 {
			domain := []string{
				split[0],
			}
			if split[2] != "no" {
				domain = append(domain, strings.Split(split[2], ",")...)
			}
			success := false
			if split[4] != "" && split[3] != "" {
				success = true
			}
			cert := &Cert{
				MainDomain: split[0],
				RootPath:   filepath.Join(self.configHome, split[0]),
				Domain:     domain,
				CA:         split[3],
				CreatedAt:  split[4],
				RenewAt:    split[5],
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
	}
	return result, nil
}
