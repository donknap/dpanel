package proxy

import (
	"encoding/json"

	"github.com/donknap/dpanel/app/ctrl/sdk/types/common"
	"github.com/donknap/dpanel/common/function"
	"github.com/gin-gonic/gin"
)

func (self *Client) CommonEnvGetList() (result common.EnvListResult, err error) {
	data, err := self.Post(function.RouterApiUri("/common/env/get-list"), nil)
	if err != nil {
		return result, err
	}
	err = json.NewDecoder(data).Decode(&result)
	return result, err
}

func (self *Client) CommonEnvSwitch(name string) error {
	_, err := self.Post(function.RouterApiUri("/common/env/switch"), gin.H{
		"name": name,
	})
	if err != nil {
		return err
	}
	return nil
}
