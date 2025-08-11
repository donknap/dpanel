//go:build windows

package exec

func WithIndependentProcessGroup() Option {
	return func(self *Command) error {
		// windows 不支持
		return nil
	}
}
