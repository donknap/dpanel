package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/donknap/dpanel/app/agent/internal"
	agentTypes "github.com/donknap/dpanel/app/agent/types"
)

var version = "dev"

func main() {
	handlers := internal.Handlers{}
	new(Provider).Register(handlers)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	args := os.Args[1:]
	if len(args) == 0 {
		<-ctx.Done()
		return
	}

	var data any
	var err error
	if selected, ok := handlers[args[0]]; !ok {
		err = fmt.Errorf("unknown operation: %s", args[0])
	} else if args[0] == "version" {
		data, err = selected.Handle(ctx, args[1:])
	} else {
		var capabilities map[string]struct{}
		capabilities, err = internal.ParseCapabilities(os.Getenv("DP_AGENT_CAPS"))
		if err == nil {
			if _, allowed := capabilities[args[0]]; !allowed {
				err = fmt.Errorf("operation %q is not authorized by DP_AGENT_CAPS", args[0])
			} else if stream, ok := selected.(internal.StreamHandler); ok {
				encoder := json.NewEncoder(os.Stdout)
				err = stream.HandleStream(ctx, args[1:], func(value any) error {
					return encoder.Encode(value)
				})
				if err == nil {
					return
				}
			} else {
				data, err = selected.Handle(ctx, args[1:])
			}
		}
	}

	message := agentTypes.Message[any]{Data: data, Code: 200}
	exitCode := 0
	if err != nil {
		message = agentTypes.Message[any]{Error: err.Error(), Code: 500}
		exitCode = 1
	}

	if err = json.NewEncoder(os.Stdout).Encode(message); err != nil {
		exitCode = 1
	}
	os.Exit(exitCode)
}
