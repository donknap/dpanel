package event

import (
	"context"
	"github.com/docker/docker/api/types/container"
)

var ContainerCreateEvent = "container_create"
var ContainerDeleteEvent = "container_delete"
var ContainerEditEvent = "container_edit"

type ContainerPayload struct {
	InspectInfo    *container.InspectResponse
	OldInspectInfo *container.InspectResponse
	Ctx            context.Context
}
