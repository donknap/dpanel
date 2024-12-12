package crontab

import (
	"log/slog"
)

type RunFunc func() error

type Option func(job *Job)

func WithRunFunc(callback RunFunc) Option {
	return func(job *Job) {
		job.runFunc = append(job.runFunc, callback)
	}
}

func New(opts ...Option) *Job {
	c := &Job{
		runFunc: make([]RunFunc, 0),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type Job struct {
	runFunc []RunFunc
}

func (self Job) Run() {
	if self.runFunc != nil {
		for _, runFunc := range self.runFunc {
			err := runFunc()
			if err != nil {
				slog.Debug("crontab crash ", "err", err.Error())
				return
			}
		}
	} else {
		slog.Debug("invalid crontab job")
	}
}
