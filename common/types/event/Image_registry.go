package event

import (
	"context"
	"github.com/donknap/dpanel/common/entity"
)

var ImageRegistryCreateEvent = "image_registry_create"
var ImageRegistryDeleteEvent = "image_registry_delete"
var ImageRegistryEditEvent = "image_registry_edit"

type ImageRegistryPayload struct {
	Registry    *entity.Registry
	OldRegistry *entity.Registry
	Ctx         context.Context
}
