package proxy

import (
	"encoding/json"

	"github.com/donknap/dpanel/app/ctrl/sdk/types/app"
	"github.com/donknap/dpanel/common/function"
)

func (self *Client) AppImageCheckUpgrade(params *app.ImageCheckUpgradeOption) (result app.ImageCheckUpgradeResult, err error) {
	data, err := self.Post(function.RouterApiUri("/app/image/check-upgrade"), params)
	if err != nil {
		return result, err
	}
	err = json.NewDecoder(data).Decode(&result)
	return result, err
}

func (self *Client) AppImageTagRemote(params *app.ImageTagRemoteOption) (result app.ImageTagRemoteResult, err error) {
	data, err := self.Post(function.RouterApiUri("/app/image/tag-sync"), params)
	if err != nil {
		return result, err
	}
	err = json.NewDecoder(data).Decode(&result)
	return result, err
}
