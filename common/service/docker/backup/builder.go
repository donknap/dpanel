package backup

import (
	"context"
	"io/fs"
	"log/slog"

	"github.com/docker/docker/api/types"
	"github.com/donknap/dpanel/common/entity"
)

func New(opts ...Option) (*Builder, error) {
	var err error
	c := &Builder{}

	c.ctx, c.ctxCancel = context.WithCancel(context.Background())
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
	ctx           context.Context
	ctxCancel     context.CancelFunc
}

func (self Builder) Context() context.Context {
	return self.ctx
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
	self.ctxCancel()
	return err
}

type Manifest struct {
	Config     string               `json:"config"`
	Image      string               `json:"image"`
	Volume     []string             `json:"volume"` // Deprecated: instead VolumeList
	Network    []string             `json:"network"`
	VolumeList []ManifestVolumeInfo `json:"volumeList"`
}

type ManifestVolumeInfo struct {
	Destination string      `json:"destination"`
	Source      string      `json:"source"`
	SavePath    string      `json:"savePath"`
	Mode        fs.FileMode `json:"type"` // file dir
}

type Info struct {
	Docker types.Version
	Backup *entity.Backup
	Extend map[string]interface{}
}
