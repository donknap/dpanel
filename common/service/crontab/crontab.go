package crontab

import (
	"errors"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

var (
	Client = NewCrontab()
)

func NewCrontab() *client {
	timeLocation := time.Local
	if os.Getenv("TZ") != "" {
		timeLocation, _ = time.LoadLocation(os.Getenv("TZ"))
	}
	specParser := NewParser()
	cronWrapper := &client{
		cron: cron.New(
			cron.WithParser(specParser),
			cron.WithLocation(timeLocation),
		),
		parser: specParser,
		jobs:   make(map[string]registeredJob),
	}
	return cronWrapper
}

type client struct {
	cron   *cron.Cron
	parser cron.ScheduleParser
	mu     sync.RWMutex
	jobs   map[string]registeredJob
}

type registeredJob struct {
	job      *Job
	entryIDs []cron.EntryID
}

func (self *client) CheckExpression(express ...string) error {
	var errs error
	for _, exp := range express {
		if _, err := self.parser.Parse(exp); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}

func (self *client) AddJob(job *Job, expressions ...string) error {
	if job == nil {
		return errors.New("invalid job")
	}
	if job.Name == "" {
		return errors.New("invalid job name")
	}
	if len(expressions) == 0 {
		return errors.New("invalid expression")
	}
	schedules := make([]cron.Schedule, 0, len(expressions))
	for _, expression := range expressions {
		if expression == "" {
			return errors.New("invalid expression")
		}
		schedule, err := self.parser.Parse(expression)
		if err != nil {
			return err
		}
		schedules = append(schedules, schedule)
	}

	self.mu.Lock()
	if current, ok := self.jobs[job.Name]; ok {
		for _, entryID := range current.entryIDs {
			self.cron.Remove(entryID)
		}
	}
	entryIDs := make([]cron.EntryID, 0, len(schedules))
	for _, schedule := range schedules {
		entryIDs = append(entryIDs, self.cron.Schedule(schedule, cron.FuncJob(func() {
			job.Run()
		})))
	}
	self.jobs[job.Name] = registeredJob{job: job, entryIDs: entryIDs}
	self.mu.Unlock()

	slog.Debug("cron add job", "name", job.Name, "next run time", self.GetNextRunTime(job.Name))
	return nil
}

func (self *client) RemoveJob(name string) {
	self.mu.Lock()
	defer self.mu.Unlock()
	current, ok := self.jobs[name]
	if !ok {
		return
	}
	for _, entryID := range current.entryIDs {
		self.cron.Remove(entryID)
	}
	delete(self.jobs, name)
}

func (self *client) GetJob(name string) (*Job, bool) {
	self.mu.RLock()
	defer self.mu.RUnlock()
	current, ok := self.jobs[name]
	if !ok || current.job == nil {
		return nil, false
	}
	return current.job, true
}

func (self *client) GetJobs(pattern string) []*Job {
	if strings.Count(pattern, "*") != 1 || !strings.HasSuffix(pattern, "*") {
		return []*Job{}
	}
	prefix := strings.TrimSuffix(pattern, "*")
	if prefix == "" {
		return []*Job{}
	}

	self.mu.RLock()
	defer self.mu.RUnlock()
	jobs := make([]*Job, 0)
	for name, current := range self.jobs {
		if strings.HasPrefix(name, prefix) && current.job != nil {
			jobs = append(jobs, current.job)
		}
	}
	return jobs
}

func (self *client) GetNextRunTime(name string) []time.Time {
	self.mu.RLock()
	current, ok := self.jobs[name]
	entryIDs := append([]cron.EntryID(nil), current.entryIDs...)
	self.mu.RUnlock()
	if !ok {
		return []time.Time{}
	}
	result := make([]time.Time, 0, len(entryIDs))
	for _, entryID := range entryIDs {
		item := self.cron.Entry(entryID)
		result = append(result, item.Next)
	}
	return result
}

func (self *client) Start() {
	self.cron.Start()
}
