package docker

import (
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/exec"
	exec2 "os/exec"
)

func (self Builder) GetRunCmd(command ...string) []exec.Option {
	return []exec.Option{
		exec.WithCommandName("docker"),
		exec.WithArgs(append(
			self.runParams,
			command...,
		)...),
	}
}

func (self Builder) GetComposeCmd(command ...string) []exec.Option {
	if _, err := exec2.LookPath("docker-compose"); err == nil {
		return []exec.Option{
			exec.WithCommandName("docker-compose"),
			exec.WithArgs(command...),
			exec.WithEnv(self.runEnv),
		}
	} else {
		return []exec.Option{
			exec.WithCommandName("docker"),
			exec.WithArgs(append(append(self.runParams, "compose"), command...)...),
		}
	}
}

func (self Builder) ExecCleanResult(str []byte) string {
	// 执行命令时返回的结果应该以 utf8 字符返回，并过滤掉不可见字符
	out := function.BytesCleanFunc(str, func(b byte) bool {
		return b < 32 && b != '\n' && b != '\r' && b != '\t'
	})
	return string(out)
}
