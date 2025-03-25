package backup

import (
	"errors"
	"os"
	"path/filepath"
)

type Option func(self *Builder) error

func WithTarPathPrefix(prefix string) Option {
	return func(self *Builder) error {
		if prefix != "" {
			self.Write.tarPathPrefix = prefix
		}
		return nil
	}
}

func WithPath(path string) Option {
	return func(self *Builder) error {
		if path == "" {
			return errors.New("invalid path")
		}
		if _, err := os.Stat(filepath.Dir(path)); err != nil {
			_ = os.MkdirAll(filepath.Dir(path), os.ModePerm)
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		self.Write.file = file
		return nil
	}
}
