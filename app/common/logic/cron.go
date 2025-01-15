package logic

import (
	"bytes"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/service/crontab"
	"github.com/donknap/dpanel/common/service/exec"
	"github.com/donknap/dpanel/common/service/plugin"
	"github.com/robfig/cron/v3"
	"io"
	"log/slog"
	"sync"
	"time"
)

const (
	CronRunDockerExec = "dockerExec"
)

var (
	lock = sync.RWMutex{}
)

type Cron struct {
}

func (self Cron) AddJob(task *entity.Cron) ([]cron.EntryID, error) {
	option := make([]crontab.Option, 0)

	expression := make([]string, 0)
	for _, item := range task.Setting.Expression {
		expression = append(expression, item.ToString())
	}

	option = append(option, crontab.WithRunFunc(func() error {
		if task.Setting.EnableRunBlock {
			if lock.TryLock() {
				defer lock.Unlock()
			} else {
				slog.Debug("task skipped")
				return nil
			}
		}
		startTime := time.Now()
		containerName := task.Setting.ContainerName

		dockerClient, err := Setting{}.GetDockerClient(task.Setting.DockerEnvName)
		if err != nil {
			return err
		}
		globalEnv := make([]string, 0)
		globalEnv = append(globalEnv, dockerClient.GetDockerEnv()...)

		for _, item := range task.Setting.Environment {
			globalEnv = append(globalEnv, fmt.Sprintf("%s=%s", item.Name, item.Value))
		}
		var out string
		if containerName == "" {
			// 如果没有指定容器，则直接在面板 shell 中执行
			cmd, _ := exec.New(
				exec.WithCommandName("/bin/sh"),
				exec.WithArgs("-c", task.Setting.Script),
				exec.WithEnv(globalEnv),
			)
			out = cmd.RunWithResult()
		} else {
			response, err := plugin.Command{}.Exec(containerName, container.ExecOptions{
				Privileged:   true,
				Tty:          true,
				AttachStdin:  false,
				AttachStdout: true,
				AttachStderr: false,
				Cmd: []string{
					"/bin/sh",
					"-c",
					task.Setting.Script,
				},
				Env: globalEnv,
			})
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
			defer response.Close()
			buffer := new(bytes.Buffer)
			_, err = io.Copy(buffer, response.Reader)
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
			out = buffer.String()
		}

		_ = dao.CronLog.Create(&entity.CronLog{
			CronID: task.ID,
			Value: &accessor.CronLogValueOption{
				Message: out,
				RunTime: startTime,
				UseTime: time.Now().Sub(startTime).Seconds(),
			},
		})
		keepLogTotal := task.Setting.KeepLogTotal
		if keepLogTotal <= 0 {
			keepLogTotal = 10
		}
		logIds := make([]int32, 0)
		_ = dao.CronLog.Where(dao.CronLog.CronID.Eq(task.ID)).Order(dao.CronLog.ID.Desc()).Limit(keepLogTotal).Pluck(dao.CronLog.ID, &logIds)
		if len(logIds) >= keepLogTotal {
			_, _ = dao.CronLog.Where(dao.CronLog.CronID.Eq(task.ID)).Where(dao.CronLog.ID.NotIn(logIds...)).Delete()
		}
		return nil
	}))

	option = append(option, crontab.WithRunFunc(func() error {
		task.Setting.NextRunTime = crontab.Wrapper.GetNextRunTime(task.Setting.JobIds...)
		_, err := dao.Cron.Updates(task)
		return err
	}))

	cronJob := crontab.New(option...)
	return crontab.Wrapper.AddJob(expression, cronJob)
}
