package logic

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/crontab"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/exec/local"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/patrickmn/go-cache"
	"github.com/robfig/cron/v3"
)

var (
	lock = sync.RWMutex{}
)

type JobContext struct {
	EventType string
	UseTime   float64
	output    string
	err       error
}

func (self JobContext) Output() string {
	return self.output
}

func (self JobContext) Err() error {
	return self.err
}

type Cron struct {
}

func (self Cron) AddCronJob(task *entity.Cron) (ids []cron.EntryID, err error) {
	cacheKey := fmt.Sprintf(storage.CacheKeyCronTaskStatus, task.ID)

	option := make([]crontab.Option, 0)
	option = append(option, crontab.WithName(fmt.Sprintf("%s:%s:%s", task.Setting.TriggerType, task.Setting.EventType, task.Title)))
	option = append(option, crontab.WithRunFunc(func(ctx *crontab.RunFuncContext) {
		if v, ok := storage.Cache.Get(cacheKey); ok {
			if task.Setting.EnableRunBlock && v == "running" {
				ctx.Err = crontab.SkipRun
				_ = dao.CronLog.Create(&entity.CronLog{
					CronID: task.ID,
					Value: &accessor.CronLogValueOption{
						Error:   "",
						RunTime: ctx.StartTime,
						UseTime: 0,
						Message: crontab.SkipRun.Error(),
					},
				})
			}
			return
		}
	}))
	option = append(option, crontab.WithRunFunc(func(ctx *crontab.RunFuncContext) {
		var err error
		var runCtx context.Context
		if task.Setting.ScriptRunTimeout > 0 {
			runCtx, _ = context.WithTimeout(context.Background(), time.Second*time.Duration(task.Setting.ScriptRunTimeout))
		}
		storage.Cache.Set(cacheKey, "running", cache.DefaultExpiration)
		defer func() {
			if err != nil {
				ctx.Err = err
			}
			storage.Cache.Delete(cacheKey)
		}()

		containerName := task.Setting.ContainerName
		dockerEnv, err := Env{}.GetEnvByName(task.Setting.DockerEnvName)
		if err != nil {
			return
		}

		globalEnv := make([]string, 0)
		globalEnv = append(globalEnv, dockerEnv.CommandEnv()...)
		globalEnv = append(globalEnv, function.PluckArrayWalk(task.Setting.Environment, func(i types.EnvItem) (string, bool) {
			// 这里需要根据传入的环境变量，再展开一次
			return fmt.Sprintf("%s=%s", i.Name, os.Expand(i.Value, func(s string) string {
				if ctx.Environment == nil {
					return i.Value
				}
				if v, _, ok := function.PluckArrayItemWalk(ctx.Environment, func(item types.EnvItem) bool {
					return item.Name == s
				}); ok {
					return v.Value
				} else {
					return i.Value
				}
			})), true
		})...)

		var script string
		if containerName != "" {
			dockerClient, err := docker.NewClientWithDockerEnv(dockerEnv)
			if err != nil {
				return
			}
			if task.Setting.EntryShell == "" {
				task.Setting.EntryShell = "/bin/sh"
			}
			defer func() {
				dockerClient.Close()
			}()
			script, err = self.scriptTemplate(&scriptTemplateParams{
				Container:     containerName,
				ScriptName:    fmt.Sprintf("%s-%d.sh", strings.Trim(containerName, "/"), task.ID),
				ScriptContent: task.Setting.Script,
				EntryShell:    task.Setting.EntryShell,
			})
			if err != nil {
				return
			}
		} else {
			script = task.Setting.Script
		}
		// 如果没有指定容器，则直接在面板 shell 中执行
		options := []local.Option{
			local.WithCommandName("/bin/sh"),
			local.WithArgs("-c", script),
			local.WithEnv(globalEnv),
		}
		// 如果有超时间，则需要独立进程，超时后强制终止掉
		if runCtx != nil {
			//options = append(options, local.WithIndependentProcessGroup())
		}
		cmd, _ := local.New(options...)
		if runCtx != nil {
			go func() {
				select {
				case <-runCtx.Done():
					slog.Debug("cron run timeout", "timeout", task.Setting.ScriptRunTimeout, "task", task)
					_ = cmd.Close()
				}
			}()
		}
		out, err := cmd.RunWithResult()
		ctx.Output = string(out)
		ctx.Err = err
		return
	}))
	option = append(option, crontab.WithRunFunc(func(ctx *crontab.RunFuncContext) {
		log := &entity.CronLog{
			CronID: task.ID,
			Value: &accessor.CronLogValueOption{
				Error:   "",
				RunTime: ctx.StartTime,
				UseTime: time.Now().Sub(ctx.StartTime).Seconds(),
				Message: ctx.Output,
			},
		}
		if ctx.Err != nil {
			log.Value.Error = ctx.Err.Error()
		}
		_ = dao.CronLog.Create(log)

		keepLogTotal := task.Setting.KeepLogTotal
		if keepLogTotal <= 0 {
			keepLogTotal = 10
		}
		logIds := make([]int32, 0)
		_ = dao.CronLog.Where(dao.CronLog.CronID.Eq(task.ID)).Order(dao.CronLog.ID.Desc()).Limit(keepLogTotal).Pluck(dao.CronLog.ID, &logIds)
		if len(logIds) >= keepLogTotal {
			_, _ = dao.CronLog.Where(dao.CronLog.CronID.Eq(task.ID)).Where(dao.CronLog.ID.NotIn(logIds...)).Delete()
		}
		return
	}))
	cronJob := crontab.New(option...)

	ids = make([]cron.EntryID, 0)
	for _, exp := range task.Setting.Expression {
		if id, err1 := crontab.Client.AddJob(exp.ToString(), cronJob); err1 == nil {
			ids = append(ids, id)
		} else {
			err = errors.Join(err1)
			crontab.Client.RemoveJob(ids...)
		}
	}

	return ids, err
}

type scriptTemplateParams struct {
	Container     string
	ScriptContent string
	EntryShell    string
	ScriptName    string
}

const scriptTemplateStr = `docker exec {{ .Container }} {{ .EntryShell }} -c 'cat > /{{ .ScriptName }} << "EOF"
{{ .ScriptContent }}
EOF
chmod +x /{{ .ScriptName }}
/{{ .ScriptName }}
rm -f /{{ .ScriptName }}'
`

func (self Cron) scriptTemplate(params *scriptTemplateParams) (string, error) {
	buffer := new(bytes.Buffer)
	tmpl := template.Must(template.New("docker-exec-script").Parse(scriptTemplateStr))
	err := tmpl.Execute(buffer, params)
	return buffer.String(), err
}
