package crontab

import (
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/donknap/dpanel/common/service/docker/types"
)

var (
	SkipRun = errors.New("skip this tasks")
)

type RunFuncContext struct {
	mu          sync.Mutex
	StartTime   time.Time
	Output      string
	Err         error
	Environment []types.EnvItem
}

type RunFunc func(ctx *RunFuncContext)

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
	Name        string
	runFunc     []RunFunc
	environment []types.EnvItem
}

func (self *Job) SetEnvironment(env []types.EnvItem) {
	self.environment = env
}

func (self *Job) Run() {
	if self.runFunc == nil {
		slog.Debug("invalid crontab job")
		return
	}

	ctx := &RunFuncContext{
		Output:      "",
		Err:         nil,
		StartTime:   time.Now(),
		Environment: self.environment,
	}

	for _, runFunc := range self.runFunc {
		func() {
			ctx.mu.Lock()
			defer ctx.mu.Unlock()
			runFunc(ctx)
			if ctx.Err != nil {
				slog.Debug("crontab crash", "err", ctx.Err.Error())
			}
		}()

		if ctx.Err != nil && errors.Is(ctx.Err, SkipRun) {
			break
		}
	}
}
