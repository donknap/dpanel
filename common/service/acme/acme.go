package acme

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/exec"
	"github.com/donknap/dpanel/common/service/exec/local"
	"github.com/donknap/dpanel/common/types/define"
)

const (
	DefaultCommandName     = "/root/.acme.sh/acme.sh"
	EnvOverrideCommandName = "DP_ACME_COMMAND_NAME"
	EnvOverrideConfigHome  = "DP_ACME_CONFIG_HOME"
)

func New(ctx context.Context, opts ...Option) (*Acme, error) {
	b := &Acme{
		commandName: DefaultCommandName,
		argv:        make([]string, 0),
		env:         make([]string, 0),
		ctx:         ctx,
	}
	if override := os.Getenv(EnvOverrideCommandName); override != "" {
		b.commandName = override
	}
	if override := os.Getenv(EnvOverrideConfigHome); override != "" {
		b.configHome = override
		b.argv = append(b.argv, "--config-home", b.configHome)
	} else {
		b.configHome = filepath.Dir(b.commandName)
	}
	b.env = append(b.env, "HTTP_PROXY="+os.Getenv("HTTP_PROXY"), "HTTPS_PROXY="+os.Getenv("HTTP_PROXY"))
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
	ctx         context.Context
}

func (self Acme) Run() (exec.Executor, error) {
	argv := append(self.argv, "--ecc")
	options := []local.Option{
		local.WithCommandName(self.commandName),
		local.WithArgs(argv...),
		local.WithCtx(self.ctx),
	}
	if !function.IsEmptyArray(self.env) {
		options = append(options, local.WithEnv(self.env))
	}
	return local.New(options...)
}

func (self Acme) Result() ([]byte, error) {
	argv := append(self.argv, "--register-account")
	options := []local.Option{
		local.WithCommandName(self.commandName),
		local.WithArgs(argv...),
		local.WithCtx(self.ctx),
	}
	if !function.IsEmptyArray(self.env) {
		options = append(options, local.WithEnv(self.env))
	}
	cmd, err := local.New(options...)
	if err != nil {
		return nil, err
	}
	return cmd.RunWithResult()
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

func (self Acme) List() ([]*Cert, error) {
	argv := append(self.argv, "--list", "--listraw")
	cmd, err := local.New(
		local.WithCommandName(self.commandName),
		local.WithArgs(argv...),
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

func (self Acme) Info(mainDomain string) (*Cert, error) {
	list, err := self.List()
	if err != nil {
		return nil, err
	}
	cert, _, ok := function.PluckArrayItemWalk(list, func(item *Cert) bool {
		return item.MainDomain == mainDomain
	})
	if !ok {
		return nil, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted)
	}
	if !cert.IsImport() {
		argv := append(self.argv, "--info", "-d", cert.MainDomain)
		if cmd, err := local.New(
			local.WithCommandName(self.commandName),
			local.WithArgs(argv...),
		); err == nil {
			if info, err := cmd.RunWithResult(); err == nil {
				item := function.PluckArrayWalk(strings.Split(string(info), "\n"), func(i string) (string, bool) {
					if k, v, exists := strings.Cut(i, "="); exists && k == "Le_Webroot" {
						return strings.Trim(v, "'"), true
					}
					return "", false
				})
				cert.DnsApi = strings.Join(item, "")
			}
		}
	}
	return cert, nil
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
