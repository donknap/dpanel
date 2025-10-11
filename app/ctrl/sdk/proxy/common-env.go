package proxy

import (
	"encoding/json"

	"github.com/donknap/dpanel/app/ctrl/sdk/types/common"
	"github.com/gin-gonic/gin"
)

func (self *Client) CommonEnvGetList() (result common.EnvListResult, err error) {
	data, err := self.Post("/api/common/env/get-list", nil)
	if err != nil {
		return result, err
	}
	err = json.NewDecoder(data).Decode(&result)
	return result, err
}

func (self *Client) CommonEnvSwitch(name string) error {
	_, err := self.Post("/api/common/env/switch", gin.H{
		"name": name,
	})
	if err != nil {
		return err
	}
	return nil
}
