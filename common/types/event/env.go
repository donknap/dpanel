package event

import "context"

const (
	EnvDeleteEvent = "env_delete"
)

type EnvPayload struct {
	Name string
	Ctx  context.Context
}
