package controller

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
	"sort"
)

func (self Swarm) TaskList(http *gin.Context) {
	type ParamsValidate struct {
		ServiceName string `json:"serviceName"`
		Status      string `json:"status"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	filter := filters.NewArgs()
	if params.ServiceName != "" {
		filter.Add("service", params.ServiceName)
	}
	if params.Status != "" {
		filter.Add("desired-state", params.Status)
	}
	list, err := docker.Sdk.Client.TaskList(docker.Sdk.Ctx, types.TaskListOptions{
		Filters: filter,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	self.JsonResponseWithoutError(http, gin.H{
		"list": list,
	})
	return
}

func (self Swarm) TaskListInNode(http *gin.Context) {
	type ParamsValidate struct {
		NodeId string `json:"nodeId" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	type resultItem struct {
		swarm.Service
		Task []swarm.Task `json:"Task"`
	}
	result := make([]resultItem, 0)

	filter := filters.NewArgs()
	filter.Add("node", params.NodeId)

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
	sort.Slice(serviceList, func(i, j int) bool {
		return serviceList[i].Spec.Name < serviceList[j].Spec.Name
	})
	for _, service := range serviceList {
		item := resultItem{
			Service: service,
			Task: function.PluckArrayWalk(taskList, func(item swarm.Task) (swarm.Task, bool) {
				if item.ServiceID == service.ID {
					return item, true
				} else {
					return swarm.Task{}, false
				}
			}),
		}
		sort.Slice(item.Task, func(i, j int) bool {
			return item.Task[i].CreatedAt.After(item.Task[j].CreatedAt)
		})
		result = append(result, item)
	}
	self.JsonResponseWithoutError(http, gin.H{
		"list": result,
	})
	return
}
