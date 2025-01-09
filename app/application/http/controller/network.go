package controller

import (
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"sort"
	"strings"
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
	networkInfo, _, err := docker.Sdk.Client.NetworkInspectWithRaw(docker.Sdk.Ctx, params.Name, network.InspectOptions{})
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

	networkList, err := docker.Sdk.Client.NetworkList(docker.Sdk.Ctx, network.ListOptions{
		Filters: filter,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	sort.Slice(networkList, func(i, j int) bool {
		return networkList[i].Created.After(networkList[j].Created)
	})
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
		Device  string `json:"device"`
		Address string `json:"address"`
	}
	type driverOption struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	type ParamsValidate struct {
		Name         string         `json:"name" binding:"required"`
		Driver       string         `json:"driver" binding:"required,oneof=bridge macvlan ipvlan overlay"`
		IpSubnet     string         `json:"ipSubnet"`
		IpGateway    string         `json:"ipGateway"`
		IpRange      string         `json:"ipRange"`
		IpAux        []ipAux        `json:"ipAux"`
		EnableIpV6   bool           `json:"enableIpV6"`
		IpV6Subnet   string         `json:"ipV6Subnet"`
		IpV6Gateway  string         `json:"ipV6Gateway"`
		IpV6Range    string         `json:"ipV6Range"`
		IpV6Aux      []ipAux        `json:"ipV6Aux"`
		DriverOption []driverOption `json:"driverOption"`
		Internal     bool           `json:"internal"`
		Attachable   bool           `json:"attachable"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	checkIpInSubnet := [][2]string{
		{
			params.IpGateway, params.IpSubnet,
		},
		{
			params.IpV6Gateway, params.IpV6Subnet,
		},
	}
	for _, item := range checkIpInSubnet {
		if item[0] == "" {
			continue
		}
		_, err := function.IpInSubnet(item[0], item[1])
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	ipAm := &network.IPAM{
		Config:  []network.IPAMConfig{},
		Options: map[string]string{},
		Driver:  "default",
	}

	ipAmConfig := network.IPAMConfig{}
	if params.IpGateway != "" {
		ipAmConfig.Gateway = params.IpGateway
	}
	if params.IpRange != "" {
		ipAmConfig.IPRange = params.IpRange
	}
	if !function.IsEmptyArray(params.IpAux) {
		ipAmConfig.AuxAddress = make(map[string]string)
		for _, item := range params.IpAux {
			ipAmConfig.AuxAddress[item.Device] = item.Address
		}
	}
	if params.IpSubnet != "" {
		ipAmConfig.Subnet = params.IpSubnet
	}
	if params.IpGateway != "" && params.IpSubnet != "" {
		ipAm.Config = append(ipAm.Config, ipAmConfig)
	}

	if params.EnableIpV6 {
		ipV6AmConfig := network.IPAMConfig{
			Gateway: params.IpV6Gateway,
			Subnet:  params.IpV6Subnet,
			IPRange: params.IpV6Range,
		}
		if !function.IsEmptyArray(params.IpV6Aux) {
			ipV6AmConfig.AuxAddress = make(map[string]string)
			for _, item := range params.IpV6Aux {
				ipV6AmConfig.AuxAddress[item.Device] = item.Address
			}
		}
		if params.IpV6Gateway != "" && params.IpV6Subnet != "" {
			ipAm.Config = append(ipAm.Config, ipV6AmConfig)
		}
	}

	option := make(map[string]string)
	if !function.IsEmptyArray(params.DriverOption) {
		for _, item := range params.DriverOption {
			option[item.Name] = item.Value
		}
	}

	result, err := docker.Sdk.Client.NetworkCreate(docker.Sdk.Ctx, params.Name, network.CreateOptions{
		EnableIPv6: function.PtrBool(params.EnableIpV6),
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
		Name           string   `json:"name" binding:"required"`
		ContainerName  string   `json:"containerName" binding:"required"`
		ContainerAlise []string `json:"containerAlise"`
		IpV4           string   `json:"ipV4"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	alise := make([]string, 0)
	for _, item := range params.ContainerAlise {
		alise = append(alise, strings.TrimPrefix(item, "/"))
	}
	err := docker.Sdk.NetworkConnect(docker.NetworkItem{
		Name:  params.Name,
		Alise: alise,
		IpV4:  params.IpV4,
	}, params.ContainerName)

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
		Key           string                   `json:"key"`
		Id            string                   `json:"id"`
		NetworkName   string                   `json:"networkName"`
		ContainerName string                   `json:"containerName"`
		NetworkInfo   network.EndpointResource `json:"networkInfo"`
		HostName      []string                 `json:"hostName"`
		Children      []containerListResult    `json:"children"`
	}

	var result []containerListResult
	i := 0
	for _, name := range params.Name {
		networkInfo, _ := docker.Sdk.Client.NetworkInspect(docker.Sdk.Ctx, name, network.InspectOptions{})
		item := containerListResult{
			NetworkName: name,
			Key:         name,
		}
		for id, resource := range networkInfo.Containers {
			temp := containerListResult{
				Id:          id,
				NetworkInfo: resource,
			}
			if containerRow, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, id); err == nil {
				if containerRow.NetworkSettings != nil {
					if networkSetting, ok := containerRow.NetworkSettings.Networks[name]; ok {
						temp.HostName = networkSetting.Aliases
					}
				}
				if containerRow.Name != "" {
					temp.ContainerName = containerRow.Name
				}
			}
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
