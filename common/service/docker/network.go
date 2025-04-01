package docker

import (
	"fmt"
	"github.com/docker/docker/api/types/network"
	"github.com/donknap/dpanel/common/function"
	"strings"
)

func (self Builder) NetworkRemove(networkName string) error {
	if networkRow, err := self.Client.NetworkInspect(self.Ctx, networkName, network.InspectOptions{}); err == nil {
		for _, item := range networkRow.Containers {
			err = self.Client.NetworkDisconnect(self.Ctx, networkName, item.Name, true)
		}
		if err != nil {
			return err
		}
		return self.Client.NetworkRemove(self.Ctx, networkName)
	}
	return nil
}

func (self Builder) NetworkCreate(networkName string, ipV4, ipV6 *NetworkCreateItem) (string, error) {
	option := network.CreateOptions{
		Driver: "bridge",
		Options: map[string]string{
			"name": networkName,
		},
		EnableIPv6: function.PtrBool(false),
		IPAM: &network.IPAM{
			Driver:  "default",
			Options: map[string]string{},
			Config:  []network.IPAMConfig{},
		},
	}
	if ipV4 != nil && ipV4.Gateway != "" && ipV4.Subnet != "" {
		option.IPAM.Config = append(option.IPAM.Config, network.IPAMConfig{
			Subnet:  ipV4.Subnet,
			Gateway: ipV4.Gateway,
		})
	}
	if ipV6 != nil && ipV6.Gateway != "" && ipV6.Subnet != "" {
		option.EnableIPv6 = function.PtrBool(true)
		option.IPAM.Config = append(option.IPAM.Config, network.IPAMConfig{
			Subnet:  ipV6.Subnet,
			Gateway: ipV6.Gateway,
		})
	}
	response, err := self.Client.NetworkCreate(self.Ctx, networkName, option)
	if err != nil {
		return "", err
	}
	return response.ID, nil
}

func (self Builder) NetworkConnect(networkRow NetworkItem, containerName string) error {
	// 关联网络时，重新退出加入
	_ = self.Client.NetworkDisconnect(self.Ctx, networkRow.Name, containerName, true)

	if networkRow.Alise == nil {
		networkRow.Alise = make([]string, 0)
	}
	dpanelHostName := fmt.Sprintf("%s.pod.dpanel.local", strings.TrimLeft(containerName, "/"))
	if !function.InArray(networkRow.Alise, dpanelHostName) {
		networkRow.Alise = append(networkRow.Alise, dpanelHostName)
	}
	endpointSetting := &network.EndpointSettings{
		Aliases:    networkRow.Alise,
		IPAMConfig: &network.EndpointIPAMConfig{},
		DNSNames:   make([]string, 0),
	}
	if networkRow.IpV4 != "" {
		endpointSetting.IPAMConfig.IPv4Address = networkRow.IpV4
	}
	if networkRow.IpV6 != "" {
		endpointSetting.IPAMConfig.IPv6Address = networkRow.IpV6
	}
	if !function.IsEmptyArray(networkRow.DnsName) {
		endpointSetting.DNSNames = networkRow.DnsName
	}
	if networkRow.MacAddress != "" {
		endpointSetting.MacAddress = networkRow.MacAddress
	}
	return self.Client.NetworkConnect(self.Ctx, networkRow.Name, containerName, endpointSetting)
}
