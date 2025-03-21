package event

import (
	"context"
	"github.com/donknap/dpanel/common/service/compose"
)

var ComposeDestroyEvent = "compose_destroy"

type ComposeDestroy struct {
	Task *compose.Task
	Ctx  context.Context
}
