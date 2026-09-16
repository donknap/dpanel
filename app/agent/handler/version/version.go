package version

import (
	"context"
	"errors"
)

type Handler struct {
	version string
}

func New(version string) *Handler {
	return &Handler{version: version}
}

func (handler *Handler) Handle(_ context.Context, args []string) (any, error) {
	if len(args) != 0 {
		return nil, errors.New("version does not accept arguments")
	}

	return map[string]string{"version": handler.version}, nil
}
