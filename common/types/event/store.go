package event

import (
	"context"

	"github.com/donknap/dpanel/common/entity"
)

var StoreDeleteEvent = "store_delete"

type StorePayload struct {
	Store *entity.Store
	Ctx   context.Context
}
