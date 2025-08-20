//go:build !windows

package local

import (
	"syscall"
)

func WithIndependentProcessGroup() Option {
	return func(self *Local) error {
		if self.cmd.SysProcAttr == nil {
			self.cmd.SysProcAttr = &syscall.SysProcAttr{}
		}
		self.cmd.SysProcAttr.Setpgid = true
		self.cmd.SysProcAttr.Pgid = 0
		return nil
	}
}
