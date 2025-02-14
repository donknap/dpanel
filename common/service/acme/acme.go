package acme

import (
	"bufio"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/exec"
	"io"
	"os"
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
	Domain    []string `json:"domain"`
	CA        string   `json:"CA"`
	CreatedAt string   `json:"createdAt"`
	RenewAt   string   `json:"renewAt"`
	Success   bool     `json:"success"`
}

func (self Acme) List() ([]Cert, error) {
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
	result := make([]Cert, 0)
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
			result = append(result, Cert{
				Domain:    domain,
				CA:        split[3],
				CreatedAt: split[4],
				RenewAt:   split[5],
				Success:   success,
			})
		}
	}
	return result, nil
}
