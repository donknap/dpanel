package docker

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

type progressStatus struct {
	Status string
}

type progressErrDetail struct {
	ErrorDetail struct {
		Message string
	} `json:"errorDetail"`
	Error string `json:"error"`
}

type progressDetail struct {
	Id             string `json:"id"`
	Status         string `json:"status"`
	ProgressDetail struct {
		Current float64 `json:"current"`
		Total   float64 `json:"total"`
	} `json:"progressDetail"`
}

type progressStream struct {
	Stream string `json:"stream"`
}

type progressAux struct {
	Aux struct {
		ID string
	}
}

type ProgressRemoteImageAux struct {
	Aux struct {
		Tag    string `json:"Tag"`
		Digest string `json:"Digest"`
		Size   int32  `json:"Size"`
	} `json:"aux"`
}

type progress struct {
	Detail *progressDetail
	Stream *progressStream
	Aux    *progressAux
	Status *progressStatus
	Err    error
}

func (self Builder) Progress(out io.ReadCloser) (progressReadChan <-chan *progress) {
	progressChan := make(chan *progress)
	go func() {
		defer close(progressChan)
		if out == nil {
			return
		}
		reader := bufio.NewReader(out)
		for {
			line, _, err := reader.ReadLine()
			fmt.Printf("%v \n", string(line))
			if err == io.EOF {
				return
			} else {
				p := &progress{}
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
