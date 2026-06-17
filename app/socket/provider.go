package socket

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"

	socketMiddleware "github.com/donknap/dpanel/app/socket/middleware"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

type Provider struct {
}

func (Provider) Register(engine *gin.Engine) {
	socketPath := facade.GetConfig().GetString("server.http.socket")
	if socketPath == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(socketPath), 0755); err != nil {
		panic(err)
	}
	if socketInfo, err := os.Stat(socketPath); err == nil {
		if socketInfo.Mode()&os.ModeSocket == 0 {
			panic(fmt.Errorf("%s exists and is not a socket", socketPath))
		}
		if err = os.Remove(socketPath); err != nil {
			panic(err)
		}
	} else if !os.IsNotExist(err) {
		panic(err)
	}
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		panic(err)
	}
	handler := http.Handler(engine)
	handler = socketMiddleware.FnnasGateway{}.Process(handler)
	go func() {
		slog.Info("http socket listen", "path", socketPath)
		if serveErr := http.Serve(listener, handler); serveErr != nil {
			slog.Error("http socket listen", "path", socketPath, "error", serveErr.Error())
		}
	}()
}
