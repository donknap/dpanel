package accessor

import (
	"github.com/docker/docker/api/types/container"
)

type SiteContainerInfoOption struct {
	Id   string
	Info container.InspectResponse
}
