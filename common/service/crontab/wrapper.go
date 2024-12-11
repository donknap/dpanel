package crontab

import (
	"github.com/robfig/cron/v3"
	"os"
	"time"
)

var (
	Wrapper = NewCronWrapper()
)

func NewCronWrapper() *wrapper {
	timeLocation := time.Local
	if os.Getenv("TZ") != "" {
		timeLocation, _ = time.LoadLocation(os.Getenv("TZ"))
	}
	specParser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	cronWrapper := &wrapper{
		Cron: cron.New(
			cron.WithParser(specParser),
			cron.WithLocation(timeLocation),
		),
		parser: specParser,
	}
	cronWrapper.Cron.Start()
	return cronWrapper
}

type wrapper struct {
	Cron   *cron.Cron
	parser cron.Parser
}

func (self wrapper) CheckExpression(expression []string) error {
	for _, exp := range expression {
		_, err := self.parser.Parse(exp)
		if err != nil {
			return err
		}
	}
	return nil
}

func (self wrapper) AddJob(expression []string, job *Job) ([]cron.EntryID, error) {
	ids := make([]cron.EntryID, 0)
	for _, item := range expression {
		id, err := self.Cron.AddJob(item, job)
		ids = append(ids, id)
		if err != nil {
			self.RemoveJob(ids...)
			return nil, err
		}
	}
	return ids, nil
}

func (self wrapper) RemoveJob(ids ...cron.EntryID) {
	for _, entryID := range ids {
		self.Cron.Remove(entryID)
	}
}

func (self wrapper) GetNextRunTime(ids ...cron.EntryID) []time.Time {
	result := make([]time.Time, 0)
	for _, entryID := range ids {
		result = append(result, self.Cron.Entry(entryID).Next)
	}
	return result
}
