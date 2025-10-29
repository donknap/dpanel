package docker

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker/stats"
	"github.com/donknap/dpanel/common/types/define"
)

func (self Builder) ContainerStats(ctx context.Context, option ContainerStatsOption) (<-chan []*stats.Usage, error) {
	containerList, err := self.Client.ContainerList(ctx, container.ListOptions{
		Filters: option.Filters,
		All:     true,
	})
	if err != nil {
		return nil, err
	}
	if function.IsEmptyArray(containerList) {
		return nil, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted)
	}

	statsChan := make(chan []*stats.Usage)

	waitFirst := &sync.WaitGroup{}
	statsCollect := stats.Collect{}

	for _, containerInfo := range containerList {
		s := &stats.Container{
			Usage: &stats.Usage{
				Container: containerInfo.ID,
				Name:      containerInfo.Names[0][1:],
			},
		}
		if statsCollect.Add(s) {
			waitFirst.Add(1)
			//time.Sleep(500 * time.Microsecond)
			go collect(ctx, s, self.Client, option.Stream, waitFirst)
		}
	}

	waitFirst.Wait()

	statsCollect.Lock()
	errs := function.PluckArrayWalk(statsCollect.List, func(item *stats.Container) (error, bool) {
		if err := item.GetError(); err != nil {
			return err, true
		} else {
			return nil, false
		}
	})
	statsCollect.Unlock()

	if !function.IsEmptyArray(errs) {
		return nil, errors.Join(errs...)
	}

	go func() {
		defer close(statsChan)

		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// 收集当前所有容器的最新统计
				statsCollect.Lock()
				var responses []*stats.Usage
				for _, c := range statsCollect.List {
					if v := c.GetStatistics(); v != nil {
						responses = append(responses, v)
					}
				}
				statsCollect.Unlock()

				// 发送统计列表
				select {
				case statsChan <- responses:
				case <-ctx.Done():
					return
				}

				if !option.Stream {
					return
				}
			}
		}
	}()

	return statsChan, nil
}

func (self Builder) ContainerStatsOneShot(ctx context.Context) ([]*stats.Usage, error) {
	result, err := self.ContainerStats(ctx, ContainerStatsOption{
		Stream: false,
	})
	if err != nil {
		return nil, err
	}
	return <-result, nil
}

func collect(ctx context.Context, containerCollect *stats.Container, sdk *client.Client, stream bool, waitFirst *sync.WaitGroup) {
	var (
		getFirst bool
	)

	defer func() {
		// if error happens, and we get nothing of stats, release wait group whatever
		if !getFirst {
			getFirst = true
			waitFirst.Done()
		}
	}()
	response, err := sdk.ContainerStats(ctx, containerCollect.Name, stream)
	if err != nil {
		containerCollect.SetError(err)
		return
	}

	dec := json.NewDecoder(response.Body)
	go func() {
		defer func() {
			_ = response.Body.Close()
		}()
		for {
			var v container.StatsResponse
			if err := dec.Decode(&v); err != nil {
				containerCollect.SetErrorAndReset()
				return
			}
			containerCollect.SetStatistics(&v, response.OSType)
			if !getFirst {
				getFirst = true
				waitFirst.Done()
			}
			if !stream {
				return
			}
		}
	}()
}
