package event

import (
	"context"
	"github.com/donknap/dpanel/common/entity"
)

var ComposeDeleteEvent = "compose_delete"

type ComposeDelete struct {
	Composes []*entity.Compose
	Ctx      context.Context
}
