package controller

import (
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/event"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type Event struct {
	controller.Abstract
}

func (self Event) GetList(http *gin.Context) {
	list := make([]*event.DockerMessagePayload, 0)
	if v, ok := storage.Cache.Get(storage.CacheKeyDockerEvents); ok {
		if t, ok := v.([]*event.DockerMessagePayload); ok {
			list = t
		}
	}
	self.JsonResponseWithoutError(http, gin.H{
		"list": list,
	})
	return
}

func (self Event) Prune(http *gin.Context) {
	storage.Cache.Delete(storage.CacheKeyDockerEvents)
	self.JsonSuccessResponse(http)
	return
}
