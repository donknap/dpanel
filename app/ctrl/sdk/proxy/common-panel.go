package proxy

import (
	"encoding/json"

	"github.com/donknap/dpanel/app/ctrl/sdk/types/common"
	"github.com/donknap/dpanel/common/function"
)

func (self *Client) CommonPanelBackup(params common.PanelBackupOption) (result common.PanelBackupResult, err error) {
	data, err := self.Post(function.RouterApiUri("/common/panel/backup"), params)
	if err != nil {
		return result, err
	}
	err = json.NewDecoder(data).Decode(&result)
	return result, err
}
