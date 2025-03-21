package event

import (
	"context"
	"github.com/donknap/dpanel/common/service/compose"
)

var ComposeDeployEvent = "compose_deploy"

type ComposeDeploy struct {
	Task *compose.Task
	Ctx  context.Context
}
