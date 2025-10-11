package event

import (
	"context"

	"github.com/donknap/dpanel/common/entity"
)

var ComposeCreateEvent = "compose_create"
var ComposeDeleteEvent = "compose_delete"

type ComposePayload struct {
	Compose *entity.Compose
	Ctx     context.Context
}
