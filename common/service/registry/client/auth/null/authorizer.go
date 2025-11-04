package null

import (
	"net/http"

	"github.com/donknap/dpanel/common/service/registry/types"
)

// NewAuthorizer returns a null authorizer
func NewAuthorizer() types.Authorizer {
	return &authorizer{}
}

type authorizer struct{}

func (a *authorizer) Modify(_ *http.Request) error {
	// do nothing
	return nil
}
