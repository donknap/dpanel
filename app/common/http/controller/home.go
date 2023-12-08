package controller

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/we7coreteam/w7-rangine-go/src/core/err_handler"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
	"net/http"
)

type Home struct {
	controller.Abstract
}

func (home Home) Index(ctx *gin.Context) {
	home.JsonResponseWithoutError(ctx, "hello world!")
	return
}

func (home Home) Ws(ctx *gin.Context) {
	wsServer := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	ws, err := wsServer.Upgrade(ctx.Writer, ctx.Request, nil)

	if err_handler.Found(err) {
		println(err.Error())
		home.JsonResponseWithoutError(ctx, err.Error())
		return
	}
	defer ws.Close()

	for {
		mt, message, err := ws.ReadMessage()
		if err != nil {
			fmt.Println(err)
			break
		}
		println(string(message))
		if string(message) == "ping" {
			message = []byte("pong")
		}
		err = ws.WriteMessage(mt, message)
		if err != nil {
			fmt.Println(err)
			break
		}
	}
}
