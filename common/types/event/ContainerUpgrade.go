package event

import (
	"context"
	"github.com/docker/docker/api/types/container"
)

var ContainerUpgradeEvent = "container_upgrade"

type ContainerUpgrade struct {
	ContainerId    string
	OldContainerId string
	InspectInfo    *container.InspectResponse
	OldInspectInfo *container.InspectResponse
	Ctx            context.Context
}
