//go:build windows

package local

func WithIndependentProcessGroup() Option {
	return func(self *Local) error {
		// windows 不支持
		return nil
	}
}

func WithKillProcessGroupOnCancel() Option {
	return func(self *Local) error {
		// windows 不支持
		return nil
	}
}
