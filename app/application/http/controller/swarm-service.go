package controller

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	swarm2 "github.com/donknap/dpanel/common/service/docker/swarm"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/registry"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
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
		SiteTitle string `json:"siteTitle"`
		SiteName  string `json:"siteName" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	buildParams := accessor.SiteEnvOption{}
	if !self.Validate(http, &buildParams) {
		return
	}

	options := make([]swarm2.Option, 0)
	options = append(options, swarm2.WithContainerSpec(&buildParams))
	options = append(options, swarm2.WithName(params.SiteName),
		swarm2.WithLabel(docker.ValueItem{
			Name:  define.SwarmLabelServiceDescription,
			Value: params.SiteTitle,
		}),
		swarm2.WithScheduling(buildParams.Scheduling),
		swarm2.WithConstraint(buildParams.Constraint),
		swarm2.WithPlacement(buildParams.Placement...),
		swarm2.WithRestart(buildParams.RestartPolicy),
		swarm2.WithPort(buildParams.Ports...),
		swarm2.WithVolume(buildParams.Volumes...),
		swarm2.WithResourceLimit(buildParams.Cpus, buildParams.Memory, 0),
	)

	if buildParams.ImageRegistry > 0 {
		if registryInfo, err := dao.Registry.Where(dao.Registry.ID.Eq(buildParams.ImageRegistry)).First(); err == nil {
			password, _ := function.AseDecode(facade.GetConfig().GetString("app.name"), registryInfo.Setting.Password)
			options = append(options, swarm2.WithRegistry(registry.Config{
				Username:   registryInfo.Setting.Username,
				Password:   password,
				ExistsAuth: true,
			}))
		}
	}

	builder, err := swarm2.New(options...)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	response, err := builder.Execute()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"id": response.ID,
	})
	return
}
