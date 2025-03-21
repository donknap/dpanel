package events

import "context"

var ContainerDeleteEvent = "container_delete"

type ContainerDelete struct {
	Name string
	Ctx  context.Context
}
