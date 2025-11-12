package controller

import (
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	swarm2 "github.com/donknap/dpanel/common/service/docker/swarm"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/gin-gonic/gin"
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
	serviceList, err := docker.Sdk.Client.ServiceList(docker.Sdk.Ctx, swarm.ServiceListOptions{
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

func (self Swarm) ServiceDetail(http *gin.Context) {
	type ParamsValidate struct {
		Id string `json:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	filter := filters.NewArgs()
	if params.Id != "" {
		filter.Add("id", params.Id)
	}
	serviceList, err := docker.Sdk.Client.ServiceList(docker.Sdk.Ctx, swarm.ServiceListOptions{
		Status:  true,
		Filters: filter,
	})
	if err != nil || len(serviceList) == 0 {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}
	item, _, ok := function.PluckArrayItemWalk(serviceList, func(item swarm.Service) bool {
		return item.ID == params.Id
	})
	if !ok {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"info": item,
	})
	return
}

func (self Swarm) ServiceScaling(http *gin.Context) {
	type ParamsValidate struct {
		Name     string `json:"name" binding:"required"`
		Force    bool   `json:"force"`
		Replicas uint64 `json:"replicas"`
		Rollback bool   `json:"rollback"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	serviceInfo, _, err := docker.Sdk.Client.ServiceInspectWithRaw(docker.Sdk.Ctx, params.Name, swarm.ServiceInspectOptions{})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	updateOptions := swarm.ServiceUpdateOptions{}

	if params.Rollback {
		updateOptions.Rollback = "previous"
	} else {
		serviceInfo.Spec.Mode = swarm.ServiceMode{
			Replicated: &swarm.ReplicatedService{
				Replicas: &params.Replicas,
			},
		}
		if params.Force {
			if serviceInfo.Spec.TaskTemplate.ContainerSpec.Labels == nil {
				serviceInfo.Spec.TaskTemplate.ContainerSpec.Labels = make(map[string]string)
			}
			serviceInfo.Spec.TaskTemplate.ContainerSpec.Labels[define.SwarmLabelServiceVersion] = time.Now().String()
		}
	}

	if serviceInfo.Spec.Labels != nil {
		if v, ok := serviceInfo.Spec.Labels[define.SwarmLabelServiceImageRegistry]; ok {
			registryConfig := logic.Image{}.GetRegistryConfig(v)
			if registryConfig != nil && registryConfig.GetAuthString() != "" {
				updateOptions.EncodedRegistryAuth = registryConfig.GetAuthString()
			}
		}
	}

	response, err := docker.Sdk.Client.ServiceUpdate(docker.Sdk.Ctx, serviceInfo.ID, serviceInfo.Version, serviceInfo.Spec, updateOptions)
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
		ServiceId string `json:"id"`
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
	if params.ServiceId != "" {
		serviceInfo, _, err := docker.Sdk.Client.ServiceInspectWithRaw(docker.Sdk.Ctx, params.ServiceId, swarm.ServiceInspectOptions{})
		if err != nil {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted, err.Error()), 500)
			return
		}
		options = append(options, swarm2.WithServiceUpdate(serviceInfo))
	}
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
			registryConfig := logic.Image{}.GetRegistryConfig(registryInfo.ServerAddress)
			options = append(options, swarm2.WithRegistryAuth(registryConfig.GetAuthString()))
			options = append(options, swarm2.WithLabel(docker.ValueItem{
				Name:  define.SwarmLabelServiceImageRegistry,
				Value: registryInfo.ServerAddress,
			}))
		}
	}

	builder, err := swarm2.New(options...)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	id, warning, err := builder.Execute()

	if function.IsEmptyArray(warning) {
		slog.Warn("swarm service", "warnging", warning)
	}

	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"id": id,
	})
	return
}
