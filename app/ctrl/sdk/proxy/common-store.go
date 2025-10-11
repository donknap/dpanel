package proxy

import (
	"encoding/json"

	"github.com/donknap/dpanel/app/ctrl/sdk/types/common"
	"github.com/donknap/dpanel/common/accessor"
)

func (self *Client) CommonStoreSync(option *common.StoreSyncOption) (list []accessor.StoreAppItem, err error) {
	data, err := self.Post("/api/common/store/sync", option)
	if err != nil {
		return list, err
	}
	result := struct {
		List []accessor.StoreAppItem `json:"list"`
	}{
		List: make([]accessor.StoreAppItem, 0),
	}
	err = json.NewDecoder(data).Decode(&result)
	return result.List, err
}
