package controller

import (
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/gin-gonic/gin"
	"sort"
	"strings"
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

func (self Swarm) ServiceScaling(http *gin.Context) {
	type ParamsValidate struct {
		Name     string `json:"name" binding:"required"`
		Mode     string `json:"mode" binding:"oneof=global replicated none"`
		Replicas uint64 `json:"replicas"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	serviceInfo, _, err := docker.Sdk.Client.ServiceInspectWithRaw(docker.Sdk.Ctx, params.Name, types.ServiceInspectOptions{})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if params.Mode == "global" {
		serviceInfo.Spec.Mode = swarm.ServiceMode{
			Global: &swarm.GlobalService{},
		}
	}
	if params.Mode == "replicated" {
		serviceInfo.Spec.Mode = swarm.ServiceMode{
			Replicated: &swarm.ReplicatedService{
				Replicas: &params.Replicas,
			},
		}
	}
	if params.Mode == "none" {
		serviceInfo.Spec.Mode = swarm.ServiceMode{
			Replicated: &swarm.ReplicatedService{
				Replicas: function.Ptr(uint64(0)),
			},
		}
	}
	response, err := docker.Sdk.Client.ServiceUpdate(docker.Sdk.Ctx, serviceInfo.ID, serviceInfo.Version, serviceInfo.Spec, types.ServiceUpdateOptions{})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if !function.IsEmptyArray(response.Warnings) {
		_ = notice.Message{}.Info(strings.Join(response.Warnings, ", "))
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Swarm) ServiceDelete(http *gin.Context) {
	type ParamsValidate struct {
		Name []string `json:"name" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	for _, name := range params.Name {
		err := docker.Sdk.Client.ServiceRemove(docker.Sdk.Ctx, name)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Swarm) ServiceCreate(http *gin.Context) {
	type ParamsValidate struct {
		SiteTitle   string `json:"siteTitle"`
		SiteName    string `json:"siteName" binding:"required"`
		ImageName   string `json:"imageName" binding:"required"`
		ContainerId string `json:"containerId"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	buildParams := accessor.SiteEnvOption{}
	if !self.Validate(http, &buildParams) {
		return
	}

	response, err := docker.Sdk.Client.ServiceCreate(docker.Sdk.Ctx, swarm.ServiceSpec{
		Annotations: swarm.Annotations{
			Name:   "",
			Labels: make(map[string]string),
		},
		TaskTemplate:   swarm.TaskSpec{},
		Mode:           swarm.ServiceMode{},
		UpdateConfig:   nil,
		RollbackConfig: nil,
		EndpointSpec:   nil,
	}, types.ServiceCreateOptions{
		EncodedRegistryAuth: "",
		QueryRegistry:       false,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	fmt.Printf("ServiceCreate %v \n", response.ID)
}
