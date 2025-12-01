package event

import (
	"context"

	"github.com/donknap/dpanel/common/entity"
)

const (
	ImageRegistryCreateEvent = "image_registry_create"
	ImageRegistryDeleteEvent = "image_registry_delete"
	ImageRegistryEditEvent   = "image_registry_edit"
)

type ImageRegistryPayload struct {
	Registry    *entity.Registry
	OldRegistry *entity.Registry
	Ctx         context.Context
}
