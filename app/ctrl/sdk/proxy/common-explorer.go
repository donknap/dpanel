package proxy

import (
	"github.com/donknap/dpanel/common/function"
	"github.com/gin-gonic/gin"
)

func (self *Client) CommonExplorerSyncDPanel(composeName string) error {
	_, err := self.Post(function.RouterApiUri("/common/explorer/sync-dpanel"), gin.H{
		"composeName": composeName,
	})
	return err
}
