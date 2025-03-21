package events

import "context"

var ContainerCreateEvent = "container_create"

type ContainerCreate struct {
	ContainerID string
	Name        string
	Ctx         context.Context
}
