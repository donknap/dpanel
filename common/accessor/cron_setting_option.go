package accessor

import (
	"fmt"
	"time"

	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/robfig/cron/v3"
)

const (
	CronUnitPreWeek     = "preWeek"
	CronUnitPreMonth    = "preMonth"
	CronUnitPreDay      = "preDay"
	CronUnitPreHour     = "preHour"
	CronUnitPreAtDay    = "preAtDay"
	CronUnitPreAtHour   = "preAtHour"
	CronUnitPreAtMinute = "preAtMinute"
	CronUnitPreAtSecond = "preAtSecond"
	CronUnitCode        = "code"
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
	case CronUnitPreWeek:
		return fmt.Sprintf("0 %s %s * * %s", self.Minutes, self.Hours, self.DayOfWeek)
	case CronUnitPreMonth:
		return fmt.Sprintf("0 %s %s %s * *", self.Minutes, self.Hours, self.DayOfMonth)
	case CronUnitPreDay:
		return fmt.Sprintf("0 %s %s * * *", self.Minutes, self.Hours)
	case CronUnitPreHour:
		return fmt.Sprintf("0 %s * * * *", self.Minutes)
	case CronUnitPreAtDay:
		return fmt.Sprintf("0 %s %s */%s * *", self.Minutes, self.Hours, self.DayOfMonth)
	case CronUnitPreAtHour:
		return fmt.Sprintf("0 %s 0-23/%s * * *", self.Minutes, self.Hours)
	case CronUnitPreAtMinute:
		return fmt.Sprintf("0 */%s * * * *", self.Minutes)
	case CronUnitPreAtSecond:
		return fmt.Sprintf("*/%s * * * * *", self.Seconds)
	case CronUnitCode:
		return self.Code
	}
	return "0 0 0 * * *"
}

const (
	CronTriggerTypeCron   = "cron"
	CronTriggerTypeManual = "manual"
	ContTriggerTypeEvent  = "event"
)

type CronSettingOption struct {
	NextRunTime      []time.Time             `json:"nextRunTime,omitempty"`
	Expression       []CronSettingExpression `json:"expression,omitempty"`
	ContainerName    string                  `json:"containerName,omitempty"`
	Script           string                  `json:"script,omitempty"`
	JobIds           []cron.EntryID          `json:"jobIds,omitempty"`
	Environment      []types.EnvItem         `json:"environment,omitempty"`
	EnableRunBlock   bool                    `json:"enableRunBlock,omitempty"`
	KeepLogTotal     int                     `json:"keepLogTotal,omitempty"`
	Disable          bool                    `json:"disable,omitempty"`
	DockerEnvName    string                  `json:"dockerEnvName,omitempty"`
	EntryShell       string                  `json:"entryShell,omitempty"`
	ScriptRunTimeout int                     `json:"scriptRunTimeout,omitempty"`
	TriggerType      string                  `json:"triggerType,omitempty"`
	EventType        string                  `json:"eventType,omitempty"`
	EventContainer   string                  `json:"eventContainer,omitempty"`
}
