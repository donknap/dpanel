package events

import "context"

var ImageRegistryEditEvent = "image_registry_edit"

type ImageRegistryEdit struct {
	OldServerAddress string
	ServerAddress    string
	Ctx              context.Context
}
