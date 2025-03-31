package event

import (
	"context"
	"github.com/donknap/dpanel/common/entity"
)

var ComposeCreateEvent = "compose_create"

type ComposeCreate struct {
	Compose *entity.Compose
	Ctx     context.Context
}
