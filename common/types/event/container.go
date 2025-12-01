package event

import (
	"context"

	"github.com/docker/docker/api/types/container"
)

const (
	ContainerCreateEvent = "container_create"
	ContainerDeleteEvent = "container_delete"
	ContainerEditEvent   = "container_edit"
)

type ContainerPayload struct {
	InspectInfo    *container.InspectResponse
	OldInspectInfo *container.InspectResponse
	Ctx            context.Context
}
