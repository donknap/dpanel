package logic

import (
	"errors"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/service/crontab"
	"github.com/donknap/dpanel/common/service/plugin"
	"github.com/robfig/cron/v3"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"time"
)

const (
	CronRunDockerExec  = "dockerExec"
	CronRunDockerShell = "dockerShell"
)

type Cron struct {
}

func (self Cron) AddJob(task *entity.Cron) error {
	option := make([]crontab.Option, 0)

	expression := make([]string, 0)
	for _, item := range task.Setting.Expression {
		expression = append(expression, item.ToString())
	}

	switch task.Setting.ScriptType {
	case CronRunDockerExec:
		option = append(option, crontab.WithRunFunc(func() error {
			startTime := time.Now()
			containerName := task.Setting.ContainerName
			if containerName == "" {
				containerName = facade.GetConfig().GetString("app.name")
			}
			out, err := plugin.Command{}.Result(containerName, task.Setting.Script)
			if err != nil {
				_ = dao.CronLog.Create(&entity.CronLog{
					CronID: task.ID,
					Value: &accessor.CronLogValueOption{
						Error:   err.Error(),
						RunTime: startTime,
					},
				})
				return err
			}
			_ = dao.CronLog.Create(&entity.CronLog{
				CronID: task.ID,
				Value: &accessor.CronLogValueOption{
					Message: out,
					RunTime: startTime,
					UseTime: time.Now().Sub(startTime).Seconds(),
				},
			})
			return nil
		}))
		break
	case CronRunDockerShell:
		option = append(option, crontab.WithRunFunc(func() error {
			startTime := time.Now()
			containerName := task.Setting.ContainerName
			if containerName == "" {
				containerName = facade.GetConfig().GetString("app.name")
			}
			out, err := plugin.Shell{}.Result(containerName, task.Setting.Script)
			if err != nil {
				_ = dao.CronLog.Create(&entity.CronLog{
					CronID: task.ID,
					Value: &accessor.CronLogValueOption{
						Error:   err.Error(),
						RunTime: startTime,
					},
				})
				return err
			}
			_ = dao.CronLog.Create(&entity.CronLog{
				CronID: task.ID,
				Value: &accessor.CronLogValueOption{
					Message: out,
					RunTime: startTime,
					UseTime: time.Now().Sub(startTime).Seconds(),
				},
			})
			return nil
		}))
	default:
		task.Setting.NextRunTime = make([]time.Time, 0)
		task.Setting.JobIds = make([]cron.EntryID, 0)
		_, _ = dao.Cron.Updates(task)

		return errors.New("unsupported crontab task type: " + task.Setting.ScriptType)
	}

	option = append(option, crontab.WithRunFunc(func() error {
		task.Setting.NextRunTime = crontab.Wrapper.GetNextRunTime(task.Setting.JobIds...)
		_, err := dao.Cron.Updates(task)
		return err
	}))

	cronJob := crontab.New(option...)

	if ids, err := crontab.Wrapper.AddJob(expression, cronJob); err == nil {
		task.Setting.NextRunTime = crontab.Wrapper.GetNextRunTime(ids...)
		task.Setting.JobIds = ids
		_, _ = dao.Cron.Updates(task)
	} else {
		task.Setting.NextRunTime = make([]time.Time, 0)
		task.Setting.JobIds = make([]cron.EntryID, 0)
		_, _ = dao.Cron.Updates(task)
		return err
	}

	return nil
}
