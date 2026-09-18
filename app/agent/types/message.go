package types

type Message[T any] struct {
	Data  T      `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
	Code  int    `json:"code"`
}
