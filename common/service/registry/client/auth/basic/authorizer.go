package basic

import (
	"net/http"

	"github.com/donknap/dpanel/common/service/registry/types"
)

// NewAuthorizer return a basic authorizer
func NewAuthorizer(username, password string) types.Authorizer {
	return &authorizer{
		username: username,
		password: password,
	}
}

type authorizer struct {
	username string
	password string
}

func (a *authorizer) Modify(req *http.Request) error {
	if len(a.username) > 0 {
		req.SetBasicAuth(a.username, a.password)
	}
	return nil
}
