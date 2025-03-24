package event

import "context"

var EnvDeleteEvent = "env_delete"

type EnvDelete struct {
	Names []string
	Ctx   context.Context
}
