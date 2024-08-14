package logic

import (
	"embed"
	"errors"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"html/template"
	"os"
	"strings"
)

const (
	LangPhp    = "php"
	LangJava   = "java"
	LangNode   = "node"
	LangGolang = "golang"
	LangHtml   = "html"
	LangOther  = "other"
)

var (
	CertFileName  = "%s.crt"
	KeyFileName   = "%s.key"
	VhostFileName = "%s.conf"
)

type Site struct {
}

func (self Site) GetEnvOptionByContainer(md5 string) (envOption accessor.SiteEnvOption, err error) {
	info, _, err := docker.Sdk.Client.ContainerInspectWithRaw(docker.Sdk.Ctx, md5, true)
	if err != nil {
		return envOption, err
	}

	if !function.IsEmptyArray(info.Config.Env) {
		for _, item := range info.Config.Env {
			if envs := strings.Split(item, "="); len(envs) > 0 {
				envOption.Environment = append(envOption.Environment, accessor.EnvItem{
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
				envOption.Links = append(envOption.Links, accessor.LinkItem{
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
			envOption.Network = append(envOption.Network, accessor.NetworkItem{
				Name:  name,
				Alise: item.Aliases,
			})
		}
	}

	if !function.IsEmptyMap(info.HostConfig.PortBindings) {
		for port, bindings := range info.HostConfig.PortBindings {
			for _, binding := range bindings {
				envOption.Ports = append(envOption.Ports, accessor.PortItem{
					HostIp: binding.HostIP,
					Host:   binding.HostPort,
					Dest:   string(port),
				})
			}
		}
	}

	if !function.IsEmptyArray(info.Mounts) {
		for _, mount := range info.Mounts {
			item := accessor.VolumeItem{
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

	envOption.Restart = string(info.HostConfig.RestartPolicy.Name)
	envOption.ImageName = info.Config.Image
	envOption.ImageId = info.Image
	envOption.Privileged = info.HostConfig.Privileged
	envOption.Cpus = int(info.HostConfig.NanoCPUs / 1000000000)
	envOption.Memory = int(info.HostConfig.Memory / 1024 / 1024)
	envOption.ShmSize = int(info.HostConfig.ShmSize)
	envOption.WorkDir = info.Config.WorkingDir
	envOption.User = info.Config.User
	envOption.Command = strings.Join(info.Config.Cmd, " ")
	envOption.Entrypoint = strings.Join(info.Config.Entrypoint, " ")

	return envOption, nil
}

func (self Site) MakeNginxConf(setting *accessor.SiteDomainSettingOption) error {
	var asset embed.FS
	err := facade.GetContainer().NamedResolve(&asset, "asset")
	if err != nil {
		return err
	}
	siteSettingPath := Site{}.GetSiteNginxSetting(setting.ServerName)

	vhostFile, err := os.OpenFile(siteSettingPath.ConfPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0666)
	if err != nil {
		return errors.New("nginx 配置目录不存在")
	}
	defer vhostFile.Close()

	if setting.SslCrt != "" && setting.SslKey != "" {
		err = os.WriteFile(siteSettingPath.CertPath, []byte(setting.SslCrt), 0666)
		if err != nil {
			return err
		}
		err = os.WriteFile(siteSettingPath.KeyPath, []byte(setting.SslKey), 0666)
		if err != nil {
			return err
		}
	}

	parser, err := template.ParseFS(asset, "asset/nginx/*.tpl")
	if err != nil {
		return err
	}
	err = parser.ExecuteTemplate(vhostFile, "vhost.tpl", setting)
	if err != nil {
		return err
	}

	return nil
}
