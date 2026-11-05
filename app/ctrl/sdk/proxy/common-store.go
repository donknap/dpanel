package proxy

import (
	"encoding/json"

	"github.com/donknap/dpanel/app/ctrl/sdk/types/common"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/function"
)

func (self *Client) CommonStoreSync(option *common.StoreSyncOption) (list []accessor.StoreAppDetail, err error) {
	data, err := self.Post(function.RouterApiUri("/common/store/sync"), option)
	if err != nil {
		return list, err
	}
	result := struct {
		List []accessor.StoreAppDetail `json:"list"`
	}{
		List: make([]accessor.StoreAppDetail, 0),
	}
	err = json.NewDecoder(data).Decode(&result)
	return result.List, err
}
