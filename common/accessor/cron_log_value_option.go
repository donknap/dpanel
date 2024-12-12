package accessor

import "time"

type CronLogValueOption struct {
	Error   string    `json:"error,omitempty"`
	Message string    `json:"message,omitempty"`
	RunTime time.Time `json:"runTime,omitempty"`
	UseTime float64   `json:"useTime,omitempty"`
}
