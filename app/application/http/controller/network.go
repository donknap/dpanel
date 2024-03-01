package controller

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
)

type Network struct {
	controller.Abstract
}

func (self Network) GetDetail(http *gin.Context) {
	type ParamsValidate struct {
		Name string `json:"name" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	networkInfo, _, err := docker.Sdk.Client.NetworkInspectWithRaw(docker.Sdk.Ctx, params.Name, types.NetworkInspectOptions{})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"info": networkInfo,
	})
	return
}

func (self Network) GetList(http *gin.Context) {
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

	networkList, err := docker.Sdk.Client.NetworkList(docker.Sdk.Ctx, types.NetworkListOptions{
		Filters: filter,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"list": networkList,
	})
	return
}

func (self Network) Prune(http *gin.Context) {
	filter := filters.NewArgs()
	_, err := docker.Sdk.Client.NetworksPrune(docker.Sdk.Ctx, filter)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Network) Delete(http *gin.Context) {
	type ParamsValidate struct {
		Name []string `json:"name" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	for _, name := range params.Name {
		err := docker.Sdk.Client.NetworkRemove(docker.Sdk.Ctx, name)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Network) Create(http *gin.Context) {
	type ipAux struct {
		IpAuxDevice  string `json:"ipAuxDevice"`
		IpAuxAddress string `json:"ipAuxAddress"`
	}
	type driverOption struct {
		DriverOptionName  string `json:"driverOptionName"`
		DriverOptionValue string `json:"driverOptionValue"`
	}
	type ParamsValidate struct {
		Name         string         `json:"name" binding:"required"`
		Driver       string         `json:"driver" binding:"required,oneof=bridge macvlan ipvlan overlay"`
		IpSubnet     string         `json:"ipSubnet"`
		IpGateway    string         `json:"ipGateway"`
		IpRange      string         `json:"ipRange"`
		IpAux        []ipAux        `json:"ipAux"`
		DriverOption []driverOption `json:"driverOption"`
		Internal     bool           `json:"internal"`
		Attachable   bool           `json:"attachable"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	ipAmConfig := network.IPAMConfig{}
	if params.IpGateway != "" {
		ipAmConfig.Gateway = params.IpGateway
	}
	if params.IpSubnet != "" {
		ipAmConfig.Subnet = params.IpSubnet
	}
	if params.IpRange != "" {
		ipAmConfig.IPRange = params.IpRange
	}
	if !function.IsEmptyArray(params.IpAux) {
		ipAmConfig.AuxAddress = make(map[string]string)
		for _, item := range params.IpAux {
			ipAmConfig.AuxAddress[item.IpAuxDevice] = item.IpAuxAddress
		}
	}

	ipAm := &network.IPAM{
		Config: []network.IPAMConfig{},
	}
	if ipAmConfig.Gateway != "" || ipAmConfig.Subnet != "" || ipAmConfig.IPRange != "" || len(ipAmConfig.AuxAddress) > 0 {
		ipAm.Config = []network.IPAMConfig{
			ipAmConfig,
		}
	}

	option := make(map[string]string)
	if !function.IsEmptyArray(params.DriverOption) {
		for _, item := range params.DriverOption {
			option[item.DriverOptionName] = item.DriverOptionValue
		}
	}

	result, err := docker.Sdk.Client.NetworkCreate(docker.Sdk.Ctx, params.Name, types.NetworkCreate{
		EnableIPv6: false,
		Driver:     params.Driver,
		IPAM:       ipAm,
		Internal:   params.Internal,
		Attachable: params.Attachable,
		Options:    option,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"id":      result.ID,
		"warning": result.Warning,
	})
	return

}

func (self Network) Disconnect(http *gin.Context) {
	type ParamsValidate struct {
		Name          string `json:"name" binding:"required"`
		ContainerName string `json:"containerName" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	err := docker.Sdk.Client.NetworkDisconnect(docker.Sdk.Ctx, params.Name, params.ContainerName, false)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Network) Connect(http *gin.Context) {
	type ParamsValidate struct {
		Name           string `json:"name" binding:"required"`
		ContainerName  string `json:"containerName" binding:"required"`
		ContainerAlise string `json:"containerAlise"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	err := docker.Sdk.Client.NetworkConnect(docker.Sdk.Ctx, params.Name, params.ContainerName, &network.EndpointSettings{
		Aliases: []string{
			params.ContainerAlise,
		},
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Network) GetContainerList(http *gin.Context) {
	type ParamsValidate struct {
		Name []string `json:"name" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	type containerListResult struct {
		Key           string                 `json:"key"`
		Id            string                 `json:"id"`
		NetworkName   string                 `json:"networkName"`
		ContainerName string                 `json:"containerName"`
		NetworkInfo   types.EndpointResource `json:"networkInfo"`
		HostName      []string               `json:"hostName"`
		Children      []containerListResult  `json:"children"`
	}

	var result []containerListResult
	i := 0
	for _, name := range params.Name {
		networkInfo, _ := docker.Sdk.Client.NetworkInspect(docker.Sdk.Ctx, name, types.NetworkInspectOptions{})
		item := containerListResult{
			NetworkName: name,
			Key:         name,
		}
		for id, resource := range networkInfo.Containers {
			temp := containerListResult{
				Id:          id,
				NetworkInfo: resource,
			}
			containerRow, _ := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, id)
			if networkSetting, ok := containerRow.NetworkSettings.Networks[name]; ok {
				temp.HostName = networkSetting.Aliases
			}
			temp.ContainerName = containerRow.Name
			temp.Key = name + ":" + temp.ContainerName
			item.Children = append(item.Children, temp)
		}
		result = append(result, item)
		i++
	}
	self.JsonResponseWithoutError(http, gin.H{
		"list": result,
	})
	return
}
