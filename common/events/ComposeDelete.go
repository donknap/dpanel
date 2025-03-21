package events

import "context"

var ComposeDeleteEvent = "compose_delete"

type ComposeDelete struct {
	Name string
	ctx  context.Context
}
