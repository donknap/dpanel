//go:build windows

package exec

func WithIndependentProcessGroup() Option {
	return func(self *Command) error {
		if self.cmd.SysProcAttr == nil {
			self.cmd.SysProcAttr = &syscall.SysProcAttr{}
		}
		// windows 不支持
		return nil
	}
}
