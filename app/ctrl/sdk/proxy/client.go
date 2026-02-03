package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/app/ctrl/sdk/types"
	"github.com/donknap/dpanel/common/function"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
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
	if os.Getenv("APP_ENV") == "debug" {
		slog.Debug("ctrl command", "uri", uri, "data", payload)
	}
	postData := new(bytes.Buffer)
	if payload == nil {
		payload = gin.H{}
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	postData.Write(jsonData)
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
			// 如果是 success 返回整个结构，如果有具体的数据，则返回数据
			switch responseMessage.Data.(type) {
			case string:
				err = json.NewEncoder(buffer).Encode(responseMessage)
			default:
				err = json.NewEncoder(buffer).Encode(responseMessage.Data)
			}
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
	privateKeyContent, err := os.ReadFile(facade.GetConfig().GetString("system.rsa.key"))
	if err != nil {
		return "", err
	}
	privateKey, err := function.RSAParsePrivateKey(privateKeyContent)
	if err != nil {
		return "", err
	}
	jwtClaims := jwt.NewWithClaims(jwt.SigningMethodRS512, logic.UserInfo{
		UserId:       currentUser.ID,
		Username:     currentUser.Value.Username,
		RoleIdentity: currentUser.Name,
		AutoLogin:    true,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(self.tokenExpire)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})
	return jwtClaims.SignedString(privateKey)
}
