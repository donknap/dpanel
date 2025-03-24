package event

import (
	"context"
	"github.com/donknap/dpanel/common/entity"
)

var ImageRegistryCreateEvent = "image_registry_create"

type ImageRegistryCreate struct {
	Registry *entity.Registry
	Ctx      context.Context
}
