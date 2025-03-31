package event

import (
	"context"
	"github.com/donknap/dpanel/common/entity"
)

var ImageRegistryDeleteEvent = "image_registry_delete"

type ImageRegistryDelete struct {
	Registries []*entity.Registry
	Ctx        context.Context
}
