//go:build windows

package local

import "fmt"

func WithWindowsCodePage() Option {
	return func(self *Local) error {
		codePage, _, err := getANSIPage.Call()
		if codePage == 0 {
			return fmt.Errorf("get Windows ANSI code page: %w", err)
		}
		self.windowsEncoding, err = windowsCodePageEncoding(codePage)
		if err != nil {
			return err
		}
		return nil
	}
}

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
