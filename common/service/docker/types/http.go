package types

import (
	"context"
	"net/http"
)

type HTTPContextInterceptor struct {
	context.Context
}

func (self HTTPContextInterceptor) Intercept(request *http.Request) error {
	*request = *request.WithContext(self.Context)
	return nil
}
