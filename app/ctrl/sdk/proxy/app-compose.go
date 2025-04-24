package proxy

import (
	"encoding/json"
	"github.com/donknap/dpanel/app/ctrl/sdk/types/app"
	"github.com/gin-gonic/gin"
)

func (self *Client) AppComposeDeploy(params *app.ComposeDeployOption) error {
	_, err := self.Post("/api/app/compose/container-deploy", params)
	if err != nil {
		return err
	}
	return nil
}

func (self *Client) AppComposeTask(name string) (result app.ComposeDetailResult, err error) {
	data, err := self.Post("/api/app/compose/get-task", gin.H{
		"id": name,
	})
	if err != nil {
		return result, err
	}
	err = json.NewDecoder(data).Decode(&result)
	return result, err
}
