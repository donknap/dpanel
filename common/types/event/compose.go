package event

import (
	"context"

	"github.com/donknap/dpanel/common/entity"
)

const (
	ComposeCreateEvent = "compose_create"
	ComposeDeleteEvent = "compose_delete"
)

type ComposePayload struct {
	Compose *entity.Compose
	Ctx     context.Context
}
