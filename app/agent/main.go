package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/donknap/dpanel/app/agent/internal"
)

var version = "dev"

func main() {
	handlers := internal.Handlers{}
	new(Provider).Register(handlers)

	args := os.Args[1:]
	if len(args) == 0 {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		<-ctx.Done()
		return
	}

	var data any
	var err error
	if selected, ok := handlers[args[0]]; !ok {
		err = fmt.Errorf("unknown operation: %s", args[0])
	} else {
		data, err = selected.Handle(context.Background(), args[1:])
	}

	message := internal.Message{Data: data, Code: 200}
	exitCode := 0
	if err != nil {
		message = internal.Message{Error: err.Error(), Code: 500}
		exitCode = 1
	}

	if err = json.NewEncoder(os.Stdout).Encode(message); err != nil {
		exitCode = 1
	}
	os.Exit(exitCode)
}
