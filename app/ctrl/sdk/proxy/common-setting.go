package proxy

import (
	"encoding/json"

	"github.com/donknap/dpanel/app/ctrl/sdk/types"
	"github.com/donknap/dpanel/app/ctrl/sdk/types/common"
	"github.com/donknap/dpanel/common/function"
)

func (self *Client) CommonCache(params common.CacheOption) (result common.CacheResult, err error) {
	data, err := self.Post(function.RouterApiUri("/common/setting/cache"), params)
	if err != nil {
		return result, err
	}
	err = json.NewDecoder(data).Decode(&result)
	return result, err
}

func (self *Client) CommonNotification(params common.NotificationOption) (result types.Message, err error) {
	data, err := self.Post(function.RouterApiUri("/common/setting/notification"), params)
	if err != nil {
		return result, err
	}
	err = json.NewDecoder(data).Decode(&result)
	return result, err
}
