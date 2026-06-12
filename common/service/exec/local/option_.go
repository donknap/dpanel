//go:build !windows

package local

import (
	"errors"
	"syscall"
	"time"
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

func WithKillProcessGroupOnCancel() Option {
	return func(self *Local) error {
		cmd := self.cmd
		cmd.Cancel = func() error {
			if cmd.Process == nil {
				return nil
			}
			err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			if errors.Is(err, syscall.ESRCH) {
				return nil
			}
			return err
		}
		cmd.WaitDelay = 3 * time.Second
		return nil
	}
}
