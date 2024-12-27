package logic

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

type message struct {
	Code  int         `json:"code"`
	Error string      `json:"error"`
	Data  interface{} `json:"data"`
}

type Proxy struct {
}

func (self Proxy) Post(uri string, token string, payload any) (responseMessage message, raw []byte, err error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return responseMessage, nil, err
	}
	uri = fmt.Sprintf("http://127.0.0.1:%s/%s", facade.GetConfig().GetString("server.http.port"), strings.Trim(uri, "/"))

	slog.Debug("cli proxy post", "uri", uri)
	request, err := http.NewRequest("POST", uri, bytes.NewBuffer(jsonData))
	if err != nil {
		return responseMessage, nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return responseMessage, nil, err
	}
	defer func() {
		_ = response.Body.Close()
	}()

	buffer := new(bytes.Buffer)
	_, _ = io.Copy(buffer, response.Body)

	if err = json.Unmarshal(buffer.Bytes(), &responseMessage); err == nil {
		switch response.StatusCode {
		case 500:
			return responseMessage, nil, errors.New(responseMessage.Error)
		case 401:
			return responseMessage, nil, errors.New("invalid auth token")
		case 200:
			return responseMessage, buffer.Bytes(), nil
		}
	} else {
		return responseMessage, nil, err
	}
	return responseMessage, nil, errors.New("unknown response status code")
}
