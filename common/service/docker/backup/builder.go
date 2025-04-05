package backup

import (
	"github.com/docker/docker/api/types"
	"github.com/donknap/dpanel/common/entity"
	"log/slog"
)

func New(opts ...Option) (*Builder, error) {
	var err error
	c := &Builder{}

	for _, opt := range opts {
		err = opt(c)
		if err != nil {
			return nil, err
		}
	}

	return c, nil
}

type Builder struct {
	tarFilePath   string
	tarPathPrefix string
	Writer        *writer
	Reader        *reader
}

func (self Builder) Close() (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = e.(error)
		}
		if self.Writer != nil {
			_ = self.Writer.tarWriter.Close()
		}
		if self.Reader != nil {
			_ = self.Reader.file.Close()
		}
	}()
	if self.Writer != nil {
		_ = self.Writer.tarWriter.Close()
		_ = self.Writer.file.Close()
	}
	if self.Reader != nil {
		err = self.Reader.file.Close()
		if err != nil {
			slog.Warn("container backup reader close file", "error", err)
		}
	}
	return err
}

type Manifest struct {
	Config  string
	Image   string
	Volume  []string
	Network []string
}

type Info struct {
	Docker types.Version
	Backup *entity.Backup
}
