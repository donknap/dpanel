package controller

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
)

func (self Swarm) TaskList(http *gin.Context) {
	type ParamsValidate struct {
		NodeName string `json:"nodeName"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	filter := filters.NewArgs()
	if params.NodeName != "" {
		filter.Add("node", params.NodeName)
	}
	list, err := docker.Sdk.Client.TaskList(docker.Sdk.Ctx, types.TaskListOptions{
		Filters: filter,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"list": list,
	})
	return
}

func (self Swarm) TaskListInNode(http *gin.Context) {
	type ParamsValidate struct {
		NodeName string `json:"nodeName" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	type resultItem struct {
		swarm.Service
		Children []swarm.Task `json:"Children"`
	}
	result := make([]resultItem, 0)

	filter := filters.NewArgs()
	if params.NodeName != "" {
		filter.Add("node", params.NodeName)
	}
	taskList, err := docker.Sdk.Client.TaskList(docker.Sdk.Ctx, types.TaskListOptions{
		Filters: filter,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	filter1 := filters.NewArgs()
	for _, task := range taskList {
		filter1.Add("id", task.ServiceID)
	}
	serviceList, err := docker.Sdk.Client.ServiceList(docker.Sdk.Ctx, types.ServiceListOptions{
		Filters: filter1,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	for _, service := range serviceList {
		item := resultItem{
			Service: service,
			Children: function.PluckArrayWalk(taskList, func(item swarm.Task) (swarm.Task, bool) {
				if item.ServiceID == service.ID {
					return item, true
				} else {
					return swarm.Task{}, false
				}
			}),
		}
		result = append(result, item)
	}
	self.JsonResponseWithoutError(http, gin.H{
		"list": result,
	})
	return
}
