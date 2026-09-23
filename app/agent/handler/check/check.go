package check

import (
	"context"
	"errors"
)

type Handler struct{}

func New() *Handler { return &Handler{} }

func (*Handler) Handle(ctx context.Context, args []string) (any, error) {
	if len(args) == 0 || args[0] != "--port" {
		return nil, errors.New("usage: check --port --target <container-id>[:<port>/<protocol>]")
	}
	return checkPorts(ctx, args[1:])
}
