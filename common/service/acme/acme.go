package acme

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/exec"
	"github.com/donknap/dpanel/common/service/exec/local"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
)

const (
	DefaultCommandName     = "/root/.acme.sh/acme.sh"
	EnvOverrideCommandName = "DP_ACME_COMMAND_NAME"
	EnvOverrideConfigHome  = define.EnvOverrideConfigHome
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
	b.configHome = storage.Local{}.GetCertDomainPath()
	b.argv = append(b.argv, "--config-home", b.configHome)
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
	return self.listExtend(self.ParseListRaw(out)), nil
}

// listExtend 补充 acme.sh --list 遗漏的证书。acme.sh 会忽略无点号域名，
// 因此这里只扫描 config home 一级目录中的 ${domain}_ecc，并要求同目录存在
// ${domain}.conf；每个缺失域名单独执行一次 --info --ecc -d，异常项直接跳过。
func (self Acme) listExtend(list []*Cert) []*Cert {
	existing := make(map[string]struct{}, len(list))
	for _, cert := range list {
		if cert != nil && cert.MainDomain != "" {
			existing[cert.MainDomain] = struct{}{}
		}
	}
	entries, err := os.ReadDir(self.configHome)
	if err != nil {
		slog.Debug("acme discover certificate directory failed", "path", self.configHome, "error", err)
		return list
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || !strings.HasSuffix(name, "_ecc") {
			continue
		}
		domain := strings.TrimSuffix(name, "_ecc")
		if domain == "" {
			continue
		}
		confPath := filepath.Join(self.configHome, name, domain+".conf")
		if info, err := os.Stat(confPath); err != nil || info.IsDir() {
			slog.Debug("acme discover certificate config missing", "domain", domain, "path", confPath, "error", err)
			continue
		}

		if _, ok := existing[domain]; ok {
			continue
		}
		// 必须按域名单独调用，避免 acme.sh 将多个 -d 合并成一次查询。
		infoArgv := append(append([]string{}, self.argv...), "--info", "--ecc", "-d", domain)
		infoCmd, err := local.New(local.WithCommandName(self.commandName), local.WithArgs(infoArgv...))
		if err != nil {
			slog.Debug("acme discover certificate info command failed", "domain", domain, "error", err)
			continue
		}
		info, err := infoCmd.RunWithResult()
		if err != nil {
			slog.Debug("acme discover certificate info failed", "domain", domain, "error", err)
			continue
		}
		values := make(map[string]string)
		scanner := bufio.NewScanner(bytes.NewReader(info))
		for scanner.Scan() {
			key, value, ok := strings.Cut(scanner.Text(), "=")
			if !ok {
				continue
			}
			key = strings.TrimSpace(key)
			switch key {
			case "Le_Domain", "Le_Alt", "Le_API", "Le_CertCreateTimeStr", "Le_NextRenewTimeStr":
				values[key] = strings.Trim(strings.TrimSpace(value), "'\"")
			}
		}
		mainDomain := strings.TrimSpace(values["Le_Domain"])
		if scanner.Err() != nil || mainDomain == "" || mainDomain != domain {
			slog.Debug("acme discover certificate info output invalid", "domain", domain)
			continue
		}
		domainList := []string{mainDomain}
		if alt := values["Le_Alt"]; alt != "" && !strings.EqualFold(alt, "no") {
			for _, item := range strings.Split(alt, ",") {
				if item = strings.TrimSpace(item); item != "" && !strings.EqualFold(item, "no") {
					domainList = append(domainList, item)
				}
			}
		}
		// --list 的 CA 列对应 --info 输出中的 Le_API，时间字段同样直接沿用原值。
		cert := &Cert{
			RootPath:   filepath.Join(self.configHome, mainDomain),
			MainDomain: mainDomain,
			Domain:     domainList,
			CA:         values["Le_API"],
			CreatedAt:  values["Le_CertCreateTimeStr"],
			RenewAt:    values["Le_NextRenewTimeStr"],
			Success:    values["Le_API"] != "" && values["Le_CertCreateTimeStr"] != "",
		}
		if _, ok := existing[cert.MainDomain]; !ok {
			list = append(list, cert)
			existing[cert.MainDomain] = struct{}{}
		}
	}
	return list
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
