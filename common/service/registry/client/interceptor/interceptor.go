package interceptor

import "net/http"

// Interceptor intercepts the request
type Interceptor interface {
	Intercept(req *http.Request) error
}
