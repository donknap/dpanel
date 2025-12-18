package crontab

import (
	"time"

	"github.com/robfig/cron/v3"
)

func NewParser() cron.ScheduleParser {
	o := Parser{
		cronParser: cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor),
	}
	return o
}

type Parser struct {
	cronParser cron.Parser
}

func (self Parser) Parse(spec string) (cron.Schedule, error) {
	if spec == "@manual" {
		return &ZeroSchedule{}, nil
	}
	return self.cronParser.Parse(spec)
}

type ZeroSchedule struct{}

func (*ZeroSchedule) Next(time.Time) time.Time {
	return time.Time{}
}
