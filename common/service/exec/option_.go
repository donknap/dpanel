//go:build !windows

package exec

import (
	"syscall"
)

func WithIndependentProcessGroup() Option {
	return func(self *Command) error {
		if self.cmd.SysProcAttr == nil {
			self.cmd.SysProcAttr = &syscall.SysProcAttr{}
		}
		self.cmd.SysProcAttr.Setpgid = true
		self.cmd.SysProcAttr.Pgid = 0
		return nil
	}
}
