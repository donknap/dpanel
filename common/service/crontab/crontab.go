package crontab

import (
	"errors"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/donknap/dpanel/common/service/docker/types"
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
		Cron: cron.New(
			cron.WithParser(specParser),
			cron.WithLocation(timeLocation),
		),
		parser: specParser,
	}
	return cronWrapper
}

type client struct {
	Cron   *cron.Cron
	parser cron.ScheduleParser
}

func (self client) CheckExpression(express ...string) error {
	var errs error
	for _, exp := range express {
		if _, err := self.parser.Parse(exp); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}

func (self client) AddJob(exp string, job *Job) (cron.EntryID, error) {
	if job == nil {
		return 0, errors.New("invalid job")
	}
	if exp == "" {
		return 0, errors.New("invalid expression")
	}
	_, err := self.parser.Parse(exp)
	if err != nil {
		return 0, err
	}
	id, err := self.Cron.AddJob(exp, job)
	slog.Debug("cron add job", "name", job.Name, "next run time", self.GetNextRunTime(id))
	return id, nil
}

func (self client) RemoveJob(ids ...cron.EntryID) {
	for _, entryID := range ids {
		self.Cron.Remove(entryID)
	}
}

func (self client) GetNextRunTime(ids ...cron.EntryID) []time.Time {
	result := make([]time.Time, 0)
	for _, entryID := range ids {
		item := self.Cron.Entry(entryID)
		result = append(result, item.Next)
	}
	return result
}

func (self client) RunById(id cron.EntryID) {
	job := self.Cron.Entry(id).Job
	v, ok := job.(*Job)
	if ok && v.Name != "" && v.runFunc != nil {
		v.Run()
	}
}

func (self client) RunByEvent(event string, env []types.EnvItem) {
	for _, entry := range self.Cron.Entries() {
		if v, ok := entry.Job.(*Job); ok && v.Name != "" && strings.HasPrefix(v.Name, "event:"+event) {
			v.SetEnvironment(env)
			v.Run()
		}
	}
}
