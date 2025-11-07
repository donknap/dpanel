package bearer

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/donknap/dpanel/common/service/registry/types"
)

// Authorizer is a kind of Modifier used to authorize the requests
type Authorizer types.Modifier

// SecretAuthorizer authorizes the requests with the specified secret
type localAuthorizer struct {
	token string
}

// NewSecretAuthorizer returns an instance of SecretAuthorizer
func NewLocalAuthorizer(token string) types.Authorizer {
	return &localAuthorizer{
		token: token,
	}
}

// Modify the request by adding secret authentication information
func (s *localAuthorizer) Modify(req *http.Request) error {
	if req == nil {
		return errors.New("the request is null")
	}

	return AddToRequest(req, s.token)
}

func AddToRequest(req *http.Request, token string) error {
	if req == nil {
		return fmt.Errorf("input request is nil, unable to set token")
	}
	req.Header.Set("Authorization", fmt.Sprintf("%s%s", "Bearer ", token))
	return nil
}
