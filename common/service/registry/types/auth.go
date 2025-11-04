package types

import (
	"net/http"
)

// Modifier modifies request
type Modifier interface {
	Modify(*http.Request) error
}

// Authorizer authorizes the request
type Authorizer Modifier
