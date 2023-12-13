package docker

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

func (self Builder) Progress(out io.ReadCloser) (progressReadChan <-chan *Progress) {
	progressChan := make(chan *Progress)
	go func() {
		defer close(progressChan)
		if out == nil {
			return
		}
		reader := bufio.NewReaderSize(out, 8192)
		for {
			line, _, err := reader.ReadLine()
			fmt.Printf("%v \n", string(line))
			if err == io.EOF {
				return
			} else {
				p := &Progress{}
				if bytes.Contains(line, []byte("errorDetail")) {
					errorDetail := &progressErrDetail{}
					err = json.Unmarshal(line, &errorDetail)
					if err != nil {
						p.Err = err
					}
					p.Err = errors.New(errorDetail.ErrorDetail.Message)
					progressChan <- p
				} else if bytes.Contains(line, []byte("{\"aux\":")) {
					aux := &progressAux{}
					err = json.Unmarshal(line, &aux)
					if err != nil {
						p.Err = err
					}
					p.Aux = aux
					progressChan <- p
				} else if bytes.Contains(line, []byte("progressDetail")) {
					pd := &progressDetail{}
					err = json.Unmarshal(line, &pd)
					if err != nil {
						p.Err = err
					}
					p.Detail = pd
					progressChan <- p
				} else if bytes.Contains(line, []byte("\"status\":")) {
					ps := &progressStatus{}
					err = json.Unmarshal(line, &ps)
					if err != nil {
						p.Err = err
					}
					p.Status = ps
					progressChan <- p
				} else if bytes.Contains(line, []byte(":\"Step")) { // 构建步骤
					stream := &progressStream{}
					err = json.Unmarshal(line, &stream)
					if err != nil {
						p.Err = err
					}
					field := strings.Fields(stream.Stream)
					step := strings.Split(field[1], "/")
					stream.Step.Total = step[1]
					stream.Step.Current = step[0]
					p.Stream = stream
					progressChan <- p
				} else {
					stream := &progressStream{}
					err = json.Unmarshal(line, &stream)
					if err != nil {
						p.Err = err
					}
					p.Stream = stream
					progressChan <- p
				}
			}
		}
		return
	}()
	return progressChan
}
