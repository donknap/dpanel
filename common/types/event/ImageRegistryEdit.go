package event

import (
	"context"
	"github.com/donknap/dpanel/common/entity"
)

var ImageRegistryEditEvent = "image_registry_edit"

type ImageRegistryEdit struct {
	OldRegistry *entity.Registry
	Registry    *entity.Registry
	Ctx         context.Context
}
