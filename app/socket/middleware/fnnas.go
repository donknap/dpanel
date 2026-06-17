package middleware

import (
	"log/slog"
	"net/http"
	"os"
	"strings"
)

type FnnasGateway struct {
}

func (FnnasGateway) Process(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if os.Getenv("DP_RUN_IN_FNNAS") != "1" || request.URL == nil {
			next.ServeHTTP(response, request)
			return
		}

		appName := strings.Trim(os.Getenv("TRIM_APPNAME"), "/")
		if appName == "" {
			appName = "dpanel"
		}
		prefix := "/app/" + appName
		if request.URL.Path == prefix || strings.HasPrefix(request.URL.Path, prefix+"/") {
			rawPath := request.URL.Path
			request.URL.Path = strings.TrimPrefix(request.URL.Path, prefix)
			if request.URL.Path == "" {
				request.URL.Path = "/"
			}
			request.URL.RawPath = ""
			slog.Debug("fnnas socket gateway rewrite", "from", rawPath, "to", request.URL.Path, "prefix", prefix)
		}

		next.ServeHTTP(response, request)
	})
}
