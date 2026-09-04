package middleware

import (
	"log/slog"
	"net/http"
	"os"
)

type FnnasGateway struct {
}

func (self FnnasGateway) Process(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if os.Getenv("DP_RUN_IN_FNNAS") != "1" || request.URL == nil {
			next.ServeHTTP(response, request)
			return
		}
		if request.Header.Get("X-Trim-Userid") != "" ||
			request.Header.Get("X-Trim-Username") != "" ||
			request.Header.Get("X-Trim-Isadmin") != "" {
			slog.Debug("fnnas socket request",
				"X-Trim-Userid", request.Header.Get("X-Trim-Userid"),
				"X-Trim-Username", request.Header.Get("X-Trim-Username"),
				"X-Trim-Isadmin", request.Header.Get("X-Trim-Isadmin"),
			)
		}
		// FNNAS forwards the gateway prefix to the Unix socket. The application
		// is configured with the same baseurl, so Gin must receive the path as-is.
		next.ServeHTTP(response, request)
	})
}
