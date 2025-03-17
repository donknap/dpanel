package controller

import (
	"bytes"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"io"
	"log/slog"
	"strconv"
)

type RunLog struct {
	controller.Abstract
}

func (self RunLog) Run(http *gin.Context) {
	type ParamsValidate struct {
		Id        string `json:"id" binding:"required"`
		LineTotal int    `json:"lineTotal" binding:"required,number,oneof=50 100 200 500 1000 5000 -1"`
		Download  bool   `json:"download"`
		ShowTime  bool   `json:"showTime"`
	}

	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	option := container.LogsOptions{
		ShowStderr: true,
		ShowStdout: true,
		Follow:     !params.Download,
		Timestamps: params.ShowTime,
	}
	if params.LineTotal > 0 {
		option.Tail = strconv.Itoa(params.LineTotal)
	}
	progress, err := ws.NewFdProgressPip(http, fmt.Sprintf(ws.MessageTypeContainerLog, params.Id))
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	if progress.IsShadow() {
		option.Follow = false
	}

	response, err := docker.Sdk.Client.ContainerLogs(docker.Sdk.Ctx, params.Id, option)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if params.Download {
		buffer, err := docker.GetContentFromStdFormat(response)
		_ = response.Close()
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		http.Header("Content-Type", "text/plain")
		http.Header("Content-Disposition", "attachment; filename="+params.Id+".log")
		http.Data(200, "text/plain", buffer.Bytes())
		return
	}

	progress.OnWrite = func(p string) error {
		newReader := bytes.NewReader([]byte(p))
		stdout := new(bytes.Buffer)
		_, err = stdcopy.StdCopy(stdout, stdout, newReader)
		if err != nil {
			progress.BroadcastMessage(p)
		} else {
			progress.BroadcastMessage(stdout.String())
		}
		return nil
	}

	go func() {
		if progress.IsShadow() {
			return
		}
		select {
		case <-progress.Done():
			slog.Debug("container", "run log response close", fmt.Sprintf(ws.MessageTypeContainerLog, params.Id))
			_ = response.Close()
		}
	}()

	_, err = io.Copy(progress, response)
	//if err != nil {
	//	self.JsonResponseWithError(http, errors.New("读取日志失败"), 500)
	//	return
	//}
	//newReader := bytes.NewReader(out.Bytes())
	//
	//stdout := new(bytes.Buffer)
	//_, err = stdcopy.StdCopy(stdout, stdout, newReader)
	//
	//if err == nil {
	//	out = stdout
	//}
	self.JsonResponseWithoutError(http, gin.H{
		"log": "",
	})
	return
}
