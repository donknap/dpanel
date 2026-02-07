package task

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"os"
	"time"

	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/donknap/dpanel/common/types/define"
)

func (self Docker) ImageSync(w *ws.ProgressPip, r io.ReadCloser) error {
	if r == nil {
		return function.ErrorMessage(define.ErrorMessageImagePullRegistryBad)
	}
	lastSendTime := time.Now()
	pg := make(map[string]*types.PullProgress)

	lastJsonStr := new(bytes.Buffer)

	w.OnWrite = func(p string) error {
		if lastJsonStr.Len() > 0 {
			p = lastJsonStr.String() + p
			lastJsonStr.Reset()
		}
		if os.Getenv("APP_ENV") == "debug" {
			slog.Debug("image pull task", "data", p)
		}
		newReader := bufio.NewReader(bytes.NewReader([]byte(p)))
		pd := types.BuildMessage{}
		for {
			line, _, err := newReader.ReadLine()
			if err == io.EOF {
				break
			}
			if err := json.Unmarshal(line, &pd); err == nil {
				if pd.ErrorDetail.Message != "" {
					return errors.New(pd.ErrorDetail.Message)
				}
				if pg[pd.Id] == nil {
					pg[pd.Id] = &types.PullProgress{
						Extracting:  0,
						Downloading: 0,
					}
				}
				if pd.ProgressDetail.Total > 0 && pd.Status == "Downloading" {
					pg[pd.Id].Downloading = math.Floor((pd.ProgressDetail.Current / pd.ProgressDetail.Total) * 100)
				}
				if pd.ProgressDetail.Total > 0 && pd.Status == "Extracting" {
					pg[pd.Id].Extracting = math.Floor((pd.ProgressDetail.Current / pd.ProgressDetail.Total) * 100)
				}
				if pd.ProgressDetail.Total > 0 && pd.Status == "Pushing" {
					pg[pd.Id].Downloading = math.Floor((pd.ProgressDetail.Current / pd.ProgressDetail.Total) * 100)
				}
				if pd.Status == "Download complete" {
					pg[pd.Id].Downloading = 100
				}
				if pd.Status == "Pull complete" {
					pg[pd.Id].Extracting = 100
					pg[pd.Id].Downloading = 100
				}
				if pd.Status == "Pushed" || pd.Status == "Layer already exists" {
					pg[pd.Id].Downloading = 100
					pg[pd.Id].Extracting = 100
				}
			} else {
				// 如果 json 解析失败，可能是最后一行 json 被截断了，存到中间变量中，下次再附加上。
				lastJsonStr.Write(line)
				slog.Debug("image pull task json", "error", err)
			}
		}
		if time.Now().Sub(lastSendTime) < time.Second {
			return nil
		}
		lastSendTime = time.Now()
		w.BroadcastMessage(pg)
		return nil
	}
	_, err := io.Copy(w, r)
	if err != nil {
		return err
	}
	w.BroadcastMessage(pg)
	return nil
}
