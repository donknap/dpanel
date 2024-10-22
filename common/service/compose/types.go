package compose

import (
	"github.com/compose-spec/compose-go/v2/types"
)

type Service struct {
	types.ServiceConfig
	XDPanelService ExtService `yaml:"x-dpanel-service,omitempty" json:"x-dpanel-service,omitempty"`
}
