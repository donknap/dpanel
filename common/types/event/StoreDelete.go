package event

import (
	"context"
	"github.com/donknap/dpanel/common/entity"
)

var StoreDeleteEvent = "store_delete"

type StoreDelete struct {
	Stores []*entity.Store
	Ctx    context.Context
}
