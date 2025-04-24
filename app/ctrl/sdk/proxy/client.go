package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/app/ctrl/sdk/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultTokenExpire = time.Minute
)

func NewProxyClient() (*Client, error) {
	c := &Client{
		tokenExpire: DefaultTokenExpire,
		apiUrl:      fmt.Sprintf("http://127.0.0.1:%s", facade.GetConfig().GetString("server.http.port")),
		ctx:         context.Background(),
	}
	return c, nil
}

type Client struct {
	tokenExpire time.Duration
	apiUrl      string
	ctx         context.Context
}

func (self *Client) Post(uri string, payload any) (data io.Reader, err error) {
	postData := new(bytes.Buffer)
	if payload != nil {
		jsonData, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		postData.Write(jsonData)
	}
	uri, err = url.JoinPath(self.apiUrl, uri)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequest("POST", uri, postData)
	if err != nil {
		return nil, err
	}
	token, err := self.token()
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)

	c := &http.Client{}
	response, err := c.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = response.Body.Close()
	}()

	responseMessage := types.Message{}
	if err = json.NewDecoder(response.Body).Decode(&responseMessage); err == nil {
		switch response.StatusCode {
		case 500:
			v, _ := json.Marshal(payload)
			return nil, fmt.Errorf("url: %s, error: %s, data: %s", strings.ReplaceAll(uri, self.apiUrl, ""), responseMessage.Error, string(v))
		case 401:
			return nil, errors.New("invalid auth token, please configure the DP_JWT_SECRET environment variable of the dpanel container")
		case 200:
			buffer := new(bytes.Buffer)
			err = json.NewEncoder(buffer).Encode(responseMessage.Data)
			return buffer, err
		}
	} else {
		return nil, err
	}
	return nil, errors.New("unknown response status code")
}

func (self *Client) token() (string, error) {
	currentUser, err := logic.Setting{}.GetValue(logic.SettingGroupUser, logic.SettingGroupUserFounder)
	if err != nil {
		return "", err
	}
	jwtSecret := logic.User{}.GetJwtSecret()
	jwtClaims := jwt.NewWithClaims(jwt.SigningMethodHS256, logic.UserInfo{
		UserId:       currentUser.ID,
		Username:     currentUser.Value.Username,
		RoleIdentity: currentUser.Name,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(self.tokenExpire)),
		},
	})
	return jwtClaims.SignedString(jwtSecret)
}
