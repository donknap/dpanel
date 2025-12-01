package event

import (
	"context"

	"github.com/donknap/dpanel/common/entity"
)

const (
	StoreDeleteEvent = "store_delete"
)

type StorePayload struct {
	Store *entity.Store
	Ctx   context.Context
}
