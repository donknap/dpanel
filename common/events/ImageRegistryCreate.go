package events

import "context"

var ImageRegistryCreateEvent = "image_registry_create"

type ImageRegistryCreate struct {
	ServerAddress string
	Ctx           context.Context
}
