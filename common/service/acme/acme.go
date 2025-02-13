package acme

import (
	"github.com/donknap/dpanel/common/service/exec"
	"io"
	"os"
)

const DefaultCommandName = "/root/.acme.sh/acme.sh"
const EnvOverrideCommandName = "ACME_HOST"

func New(opts ...Option) (*Acme, error) {
	b := &Acme{
		commandName: DefaultCommandName,
		argv:        make([]string, 0),
	}
	if override := os.Getenv(EnvOverrideCommandName); override != "" {
		b.commandName = override
	}
	for _, opt := range opts {
		if p := opt(); p != nil {
			b.argv = append(b.argv, p...)
		}
	}
	return b, nil
}

type Acme struct {
	commandName string
	argv        []string
}

func (self Acme) Run() (io.ReadCloser, error) {
	cmd, err := exec.New(
		exec.WithCommandName(self.commandName),
		exec.WithArgs(self.argv...),
	)
	if err != nil {
		return nil, err
	}
	return cmd.RunInPip()
}
