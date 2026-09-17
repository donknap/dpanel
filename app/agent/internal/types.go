package internal

import "context"

type Handler interface {
	Handle(context.Context, []string) (any, error)
}

type Handlers map[string]Handler

type StreamHandler interface {
	HandleStream(context.Context, []string, func(any) error) error
}

type Message struct {
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
	Code  int    `json:"code"`
}
