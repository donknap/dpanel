package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"github.com/docker/docker/api/types"
)

func New(opts ...Option) (*Builder, error) {
	var err error
	c := &Builder{
		Write: &write{},
	}
	c.ctx, c.cancel = context.WithCancel(context.Background())

	for _, opt := range opts {
		err = opt(c)
		if err != nil {
			return nil, err
		}
	}

	gzWriter := gzip.NewWriter(c.Write.file)
	c.Write.tarWriter = tar.NewWriter(gzWriter)

	go func() {
		select {
		case <-c.ctx.Done():
			_ = c.Write.tarWriter.Close()
			_ = gzWriter.Close()
			_ = c.Write.file.Close()
		}
	}()

	return c, nil
}

type Builder struct {
	Write  *write
	ctx    context.Context
	cancel context.CancelFunc
}

func (self Builder) Close() {
	self.cancel()
}

type Manifest struct {
	ServerVersion types.Version
	Config        string
	Volume        []string
	Image         string
}
