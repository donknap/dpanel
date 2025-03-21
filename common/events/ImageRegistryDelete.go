package events

import "context"

var ImageRegistryDeleteEvent = "image_registry_delete"

type ImageRegistryDelete struct {
	ServerAddresses []string
	Ctx             context.Context
}
