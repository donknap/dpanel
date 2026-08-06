package crontab

import (
	"log/slog"
	"time"

	"github.com/donknap/dpanel/common/service/docker/types"
)

type RunFuncContext struct {
	StartTime   time.Time
	Output      string
	Err         error
	Environment []types.EnvItem
}

type RunFunc func(ctx *RunFuncContext)

type RunOption func(options *runOptions)

type runOptions struct {
	environment []types.EnvItem
}

func WithEnvironment(environment []types.EnvItem) RunOption {
	environment = append([]types.EnvItem(nil), environment...)
	return func(options *runOptions) {
		options.environment = append([]types.EnvItem(nil), environment...)
	}
}

type Option func(job *Job)

func WithRunFunc(callback RunFunc) Option {
	return func(job *Job) {
		job.runFunc = append(job.runFunc, callback)
	}
}

func WithName(name string) Option {
	return func(job *Job) {
		job.Name = name
	}
}

func WithSkipIfStillRunning() Option {
	return func(job *Job) {
		job.skipIfStillRunning = true
	}
}

func New(opts ...Option) *Job {
	c := &Job{
		runFunc: make([]RunFunc, 0),
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.skipIfStillRunning {
		c.runGate = make(chan struct{}, 1)
		c.runGate <- struct{}{}
	}
	return c
}

type Job struct {
	Name               string
	runFunc            []RunFunc
	skipIfStillRunning bool
	runGate            chan struct{}
}

func (self *Job) Run(opts ...RunOption) {
	if self.runFunc == nil {
		slog.Debug("invalid crontab job")
		return
	}
	if self.runGate != nil {
		select {
		case token := <-self.runGate:
			defer func() {
				self.runGate <- token
			}()
		default:
			cronLogger{name: self.Name}.Info("skip")
			return
		}
	}

	options := &runOptions{}
	for _, opt := range opts {
		opt(options)
	}

	ctx := &RunFuncContext{
		Output:      "",
		Err:         nil,
		StartTime:   time.Now(),
		Environment: options.environment,
	}

	for _, runFunc := range self.runFunc {
		runFunc(ctx)
		if ctx.Err != nil {
			slog.Debug("crontab crash", "err", ctx.Err.Error())
		}
	}
}
