package controller

import (
	"sort"
	"strings"

	"github.com/donknap/dpanel/app/common/logic"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type Plugin struct {
	controller.Abstract
}

func (self Plugin) GetList(http *gin.Context) {
	var params struct {
		Source  string `json:"source"`
		Keyword string `json:"keyword"`
	}
	if !self.Validate(http, &params) {
		return
	}
	sourceName := strings.TrimSpace(params.Source)
	keyword := strings.ToLower(strings.TrimSpace(params.Keyword))

	sources := []logic.PluginSource{logic.SystemPluginSource}
	// TODO: 从数据库读取插件源并追加到 sources。
	items := make([]logic.PluginItem, 0)
	for i := range sources {
		if sourceName != "" && sources[i].Name != sourceName {
			continue
		}
		found, err := sources[i].ReadList()
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		for _, item := range found {
			if keyword != "" {
				matched := strings.Contains(strings.ToLower(item.Name), keyword)
				for _, title := range item.Title {
					if strings.Contains(strings.ToLower(title), keyword) {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			}
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Source+"/"+items[i].Name < items[j].Source+"/"+items[j].Name
	})
	self.JsonResponseWithoutError(http, gin.H{"list": items})
}
