package accessor

import (
	"fmt"
	"github.com/robfig/cron/v3"
	"time"
)

type CronSettingExpression struct {
	Unit       string `json:"unit"`
	Code       string `json:"code,omitempty"`
	Seconds    string `json:"seconds,omitempty"`
	Minutes    string `json:"minutes,omitempty"`
	Hours      string `json:"hours,omitempty"`
	DayOfMonth string `json:"dayOfMonth,omitempty"`
	Month      string `json:"month,omitempty"`
	DayOfWeek  string `json:"dayOfWeek,omitempty"`
}

func (self CronSettingExpression) ToString() string {
	switch self.Unit {
	case "preWeek":
		return fmt.Sprintf("* %s %s * * %s", self.Minutes, self.Hours, self.DayOfWeek)
	case "preMonth":
		return fmt.Sprintf("* %s %s %s * *", self.Minutes, self.Hours, self.DayOfMonth)
	case "preDay":
		return fmt.Sprintf("* %s %s * * *", self.Minutes, self.Hours)
	case "preHour":
		return fmt.Sprintf("* %s * * * *", self.Minutes)
	case "preAtDay":
		return fmt.Sprintf("* %s %s */%s * *", self.Minutes, self.Hours, self.DayOfMonth)
	case "preAtHour":
		return fmt.Sprintf("* %s 0-23/%s * * *", self.Minutes, self.Hours)
	case "preAtMinute":
		return fmt.Sprintf("* */%s * * * *", self.Minutes)
	case "preAtSecond":
		return fmt.Sprintf("*/%s * * * * *", self.Seconds)
	case "code":
		return self.Code
	}
	return "0 0 0 * * *"
}

type CronSettingOption struct {
	NextRunTime    []time.Time             `json:"nextRunTime,omitempty"`
	Expression     []CronSettingExpression `json:"expression,omitempty"`
	ContainerName  string                  `json:"containerName,omitempty"`
	Script         string                  `json:"script,omitempty"`
	JobIds         []cron.EntryID          `json:"jobIds,omitempty"`
	Environment    []EnvItem               `json:"environment,omitempty"`
	EnableRunBlock bool                    `json:"enableRunBlock,omitempty"`
	KeepLogTotal   int                     `json:"keepLogTotal,omitempty"`
	Disable        bool                    `json:"disable,omitempty"`
}
