package controller

import (
	"fmt"

	"github.com/donknap/dpanel/app/common/logic"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type PluginTap struct {
	controller.Abstract
}

func (self PluginTap) GetList(http *gin.Context) {
	sources := []logic.PluginSource{logic.SystemPluginSource}
	// TODO: 从数据库读取插件源并追加到 sources。
	self.JsonResponseWithoutError(http, gin.H{"list": sources})
}

func (self PluginTap) Sync(http *gin.Context) {
	var params struct {
		Source string `json:"source"`
	}
	if !self.Validate(http, &params) {
		return
	}
	plugin := logic.Plugin{}
	sources := []logic.PluginSource{logic.SystemPluginSource}
	results := make([]logic.PluginSyncResult, 0, len(sources))
	found := false
	for _, source := range sources {
		if params.Source != "" && params.Source != source.Source {
			continue
		}
		found = true
		result, err := plugin.Sync(http.Request.Context(), source)
		results = append(results, result)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	if !found {
		self.JsonResponseWithError(http, fmt.Errorf("plugin source not found"), 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{"results": results})
}
