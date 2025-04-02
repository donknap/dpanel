package event

import "context"

var EnvDeleteEvent = "env_delete"

type EnvPayload struct {
	Name string
	Ctx  context.Context
}
