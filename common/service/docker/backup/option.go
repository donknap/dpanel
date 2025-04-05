package backup

import (
	"archive/tar"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type Option func(self *Builder) error

func WithTarPathPrefix(prefix string) Option {
	return func(self *Builder) error {
		if prefix != "" {
			self.tarPathPrefix = strings.TrimLeft(prefix, "/")
		}
		return nil
	}
}

func WithPath(path string) Option {
	return func(self *Builder) error {
		if path == "" {
			return errors.New("invalid path")
		}
		self.tarFilePath = path
		return nil
	}
}

func WithWriter() Option {
	return func(self *Builder) error {
		dir := filepath.Dir(self.tarFilePath)
		if _, err := os.Stat(dir); err != nil {
			_ = os.MkdirAll(dir, os.ModePerm)
		}
		file, err := os.OpenFile(self.tarFilePath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		self.Writer = &writer{
			tarPathPrefix: self.tarPathPrefix,
			file:          file,
		}
		self.Writer.tarWriter = tar.NewWriter(file)
		return nil
	}
}

func WithReader() Option {
	return func(self *Builder) error {
		var err error
		if _, err := os.Stat(self.tarFilePath); err != nil {
			return err
		}
		file, err := os.OpenFile(self.tarFilePath, os.O_RDWR, 0o644)
		if err != nil {
			return err
		}
		self.Reader = &reader{
			file: file,
		}
		if err != nil {
			return err
		}
		return nil
	}
}
