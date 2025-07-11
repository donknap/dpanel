package controller

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
	"sort"
)

func (self Swarm) ServiceList(http *gin.Context) {
	type ParamsValidate struct {
		Name string `json:"name"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	filter := filters.NewArgs()
	if params.Name != "" {
		filter.Add("name", params.Name)
	}
	serviceList, err := docker.Sdk.Client.ServiceList(docker.Sdk.Ctx, types.ServiceListOptions{
		Status:  true,
		Filters: filter,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	sort.Slice(serviceList, func(i, j int) bool {
		return serviceList[i].Spec.Name < serviceList[j].Spec.Name
	})
	self.JsonResponseWithoutError(http, gin.H{
		"list": serviceList,
	})
	return
}

func (self Swarm) ServiceUpdate(gin *gin.Context) {

}
