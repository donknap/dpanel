package logic

import (
	"embed"
	"errors"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/go-units"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

var (
	CertFileName  = "fullchain.cer"
	KeyFileName   = "%s.key"
	VhostFileName = "%s.conf"
	CertName      = "%s_ecc"
	DefaultDnsApi = []accessor.DnsApi{
		{
			Title:      "Nginx",
			ServerName: "nginx",
			Env:        make([]types.EnvItem, 0),
		},
	}
)

type Site struct {
}

func (self Site) GetEnvOptionByContainer(md5 string) (envOption accessor.SiteEnvOption, err error) {
	info, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, md5)
	if err != nil {
		return envOption, err
	}

	if !function.IsEmptyArray(info.Config.Env) {
		for _, item := range info.Config.Env {
			if envs := strings.Split(item, "="); len(envs) > 0 {
				envOption.Environment = append(envOption.Environment, types.EnvItem{
					Name:  envs[0],
					Value: envs[1],
				})
			}
		}
	}

	// 关联信息，统一转化为 network 来处理
	if !function.IsEmptyArray(info.HostConfig.Links) {
		for _, item := range info.HostConfig.Links {
			if temp := strings.Split(item, ":"); len(temp) > 0 {
				envOption.Links = append(envOption.Links, types.LinkItem{
					Name:  temp[0],
					Alise: temp[1][len(info.Name) : len(temp[1])-1],
				})
			}
		}
	}
	if !function.IsEmptyMap(info.NetworkSettings.Networks) {
		for name, item := range info.NetworkSettings.Networks {
			if name == "bridge" {
				continue
			}
			network := types.NetworkItem{
				Name:  name,
				Alise: item.Aliases,
			}
			if item.IPAMConfig != nil {
				network.IpV4 = item.IPAMConfig.IPv4Address
				network.IpV6 = item.IPAMConfig.IPv6Address
			}
			envOption.Network = append(envOption.Network, network)
		}
	}

	if !function.IsEmptyMap(info.HostConfig.PortBindings) {
		for port, bindings := range info.HostConfig.PortBindings {
			for _, binding := range bindings {
				envOption.Ports = append(envOption.Ports, types.PortItem{
					HostIp: binding.HostIP,
					Host:   binding.HostPort,
					Dest:   string(port),
				})
			}
		}
	}

	if !function.IsEmptyArray(info.Mounts) {
		for _, mount := range info.Mounts {
			item := types.VolumeItem{
				Host: "",
				Dest: mount.Destination,
			}
			if mount.RW {
				item.Permission = "write"
			} else {
				item.Permission = "read"
			}
			switch mount.Type {
			case "volume":
				item.Host = mount.Name
			case "bind":
				item.Host = mount.Source
			}
			envOption.Volumes = append(envOption.Volumes, item)
		}
	}
	envOption.DockerEnvName = docker.Sdk.Name
	envOption.ImageName = info.Config.Image
	envOption.ImageId = info.Image
	envOption.Privileged = info.HostConfig.Privileged
	envOption.AutoRemove = info.HostConfig.AutoRemove
	envOption.RestartPolicy = &types.RestartPolicy{
		Name:       string(info.HostConfig.RestartPolicy.Name),
		MaxAttempt: info.HostConfig.RestartPolicy.MaximumRetryCount,
	}
	envOption.Cpus = float32(info.HostConfig.NanoCPUs / 1000000000)
	envOption.Memory = int(info.HostConfig.Memory / 1024 / 1024)
	envOption.ShmSize = units.BytesSize(float64(info.HostConfig.ShmSize))
	envOption.WorkDir = info.Config.WorkingDir
	envOption.User = info.Config.User
	envOption.Command = strings.Join(info.Config.Cmd, " ")
	envOption.Entrypoint = strings.Join(info.Config.Entrypoint, " ")
	envOption.UseHostNetwork = info.HostConfig.NetworkMode.IsHost()
	envOption.Log = &types.LogDriverItem{
		Driver:  info.HostConfig.LogConfig.Type,
		MaxFile: info.HostConfig.LogConfig.Config["max-file"],
		MaxSize: info.HostConfig.LogConfig.Config["max-size"],
	}
	envOption.Dns = info.HostConfig.DNS
	envOption.PublishAllPorts = info.HostConfig.PublishAllPorts
	envOption.ExtraHosts = make([]types.ValueItem, 0)
	for _, host := range info.HostConfig.ExtraHosts {
		value := strings.Split(host, ":")
		envOption.ExtraHosts = append(envOption.ExtraHosts, types.ValueItem{
			Name:  value[0],
			Value: value[1],
		})
	}
	if !function.IsEmptyMap(info.Config.Labels) {
		envOption.Label = make([]types.ValueItem, 0)
		for key, value := range info.Config.Labels {
			envOption.Label = append(envOption.Label, types.ValueItem{
				Name:  key,
				Value: value,
			})
		}
	}

	envOption.Device = make([]types.DeviceItem, 0)
	for _, device := range info.HostConfig.Devices {
		envOption.Device = append(envOption.Device, types.DeviceItem{
			Host: device.PathOnHost,
			Dest: device.PathInContainer,
		})
	}

	if info.Config != nil && info.Config.Healthcheck != nil {
		envOption.Healthcheck = &types.HealthcheckItem{
			ShellType: info.Config.Healthcheck.Test[0],
			Cmd:       strings.Join(info.Config.Healthcheck.Test[1:], " "),
			Interval:  int(info.Config.Healthcheck.Interval.Seconds()),
			Timeout:   int(info.Config.Healthcheck.Timeout.Seconds()),
			Retries:   info.Config.Healthcheck.Retries,
		}
	}

	if info.HostConfig.PidMode.IsHost() {
		envOption.HostPid = true
	}

	if info.HostConfig.CapAdd == nil {
		envOption.CapAdd = function.DefaultCapabilities()
	} else {
		envOption.CapAdd = info.HostConfig.CapAdd
	}

	return envOption, nil
}

func (self Site) MakeNginxConf(setting accessor.SiteDomainSettingOption) error {
	var asset embed.FS
	err := facade.GetContainer().NamedResolve(&asset, "asset")
	if err != nil {
		return err
	}
	parser, err := template.ParseFS(asset, "asset/nginx/*.tpl")
	if err != nil {
		return err
	}
	nginxConfPath := filepath.Join(storage.Local{}.GetNginxSettingPath(), fmt.Sprintf(VhostFileName, setting.ServerName))
	vhostFile, err := os.OpenFile(nginxConfPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0666)
	if err != nil {
		return errors.New("the Nginx configuration directory does not exist")
	}
	defer func() {
		_ = vhostFile.Close()
	}()
	err = parser.ExecuteTemplate(vhostFile, "vhost.tpl", setting)
	if err != nil {
		return err
	}
	err = self.MakeNginxResolver()
	return err
}

func (self Site) MakeNginxResolver() error {
	var asset embed.FS
	err := facade.GetContainer().NamedResolve(&asset, "asset")
	if err != nil {
		return err
	}
	parser, err := template.ParseFS(asset, "asset/nginx/*.tpl")
	if err != nil {
		return err
	}
	resolverFile, err := os.OpenFile("/etc/nginx/conf.d/include/resolver.conf", os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0666)
	if err != nil {
		return fmt.Errorf("create Nginx resolver configuration failed %s", err.Error())
	}
	defer func() {
		_ = resolverFile.Close()
	}()
	err = parser.ExecuteTemplate(resolverFile, "resolver.tpl", map[string]interface{}{
		"Resolver": function.SystemResolver("127.0.0.11", "1.1.1.1", "8.8.8.8"),
	})
	if err != nil {
		return err
	}
	return nil
}
