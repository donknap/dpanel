package proxy

import (
	"encoding/json"
	"github.com/donknap/dpanel/app/ctrl/sdk/types/app"
	"github.com/gin-gonic/gin"
)

func (self *Client) AppContainerGetDetail(containerName string) (result app.ContainerDetailResult, err error) {
	data, err := self.Post("/api/app/container/get-detail", gin.H{
		"md5": containerName,
	})
	if err != nil {
		return result, err
	}
	err = json.NewDecoder(data).Decode(&result)
	return result, err
}

func (self *Client) AppContainerUpgrade(params *app.ContainerUpgradeOption) (result app.ContainerUpgradeResult, err error) {
	data, err := self.Post("/api/app/container/upgrade", params)
	if err != nil {
		return result, err
	}
	err = json.NewDecoder(data).Decode(&result)
	return result, err
}
