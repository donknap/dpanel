package runner

import (
	"context"
	"io"
)

type Runner interface {
	Run(ctx context.Context, args ...string) ([]byte, error)
	Stream(ctx context.Context, args ...string) (Session, error)
}

type Session interface {
	io.Reader
	io.Writer
	io.Closer
	CloseWrite() error
}
