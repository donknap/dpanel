package logic

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/docker/docker/api/types/registry"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/we7coreteam/registry-go-sdk/types"
)

type Registry struct {
	config  registry.AuthConfig
	Address []string
}

// AuthString Docker pull/push 权限
func (self Registry) AuthString() string {
	authString, err := registry.EncodeAuthConfig(self.config)
	if err != nil {
		slog.Debug("get registry auth string", err.Error())
		return ""
	}
	return authString
}

// Credential registry 仓库权限
func (self Registry) Credential() types.Credential {
	return types.Credential{
		AccessKey:    self.config.Username,
		AccessSecret: self.config.Password,
	}
}

type Image struct {
}

const buildxConfigTmpl = `
{{- if .WorkerNetworkMode }}
[worker.oci]
  networkMode = {{ quote .WorkerNetworkMode }}

{{- end }}
{{- if .DriverHttpProxy }}
[proxy]
  httpProxy = {{ quote .DriverHttpProxy }}
  httpsProxy = {{ quote .DriverHttpProxy }}

{{- end }}
{{- range .Registry }}
[registry.{{ quote .ServerAddress }}]
{{- if .Mirrors }}
  mirrors = [{{ range $i, $mirror := .Mirrors }}{{ if $i }}, {{ end }}{{ quote $mirror }}{{ end }}]
{{- end }}
{{- if .EnableHttp }}
  http = true
{{- end }}

{{- end }}
`

type BuildxConfig struct {
	ConfigPath        string
	ConfigContent     *string
	WorkerNetworkMode string
	Registry          []BuildxConfigRegistry
	DriverHttpProxy   string
}

type BuildxConfigRegistry struct {
	ServerAddress string
	Mirrors       []string
	EnableHttp    bool
}

func (self Image) BuildxConfig(dockerEnvName string) (BuildxConfig, error) {
	buildxDefaultConfig := BuildxConfig{
		ConfigPath:        filepath.Join(storage.Local{}.GetStorageLocalPath(), "buildx", dockerEnvName, "config.toml"),
		WorkerNetworkMode: "host",
		Registry:          make([]BuildxConfigRegistry, 0),
	}
	registryRows, err := dao.Registry.Order(dao.Registry.ServerAddress.Asc()).Find()
	if err != nil {
		return buildxDefaultConfig, err
	}
	for _, row := range registryRows {
		if row == nil || row.Setting == nil || row.ServerAddress == "" {
			continue
		}
		mirrors := make([]string, 0)
		for _, proxy := range row.Setting.Proxy {
			mirror := strings.TrimSpace(proxy)
			if strings.HasPrefix(strings.ToLower(mirror), "http://") {
				mirror = mirror[len("http://"):]
			} else if strings.HasPrefix(strings.ToLower(mirror), "https://") {
				mirror = mirror[len("https://"):]
			}
			mirror = strings.TrimRight(mirror, "/")
			if mirror == "" {
				continue
			}
			mirrorHost := strings.Split(mirror, "/")[0]
			mirrorHost = strings.TrimSuffix(strings.TrimSuffix(strings.ToLower(mirrorHost), ":443"), ":80")
			if mirrorHost == define.RegistryDefaultName || mirrorHost == "index.docker.io" || mirrorHost == "registry-1.docker.io" {
				continue
			}
			mirrors = append(mirrors, mirror)
		}
		if function.IsEmptyArray(mirrors) && !row.Setting.EnableHttp {
			continue
		}
		buildxDefaultConfig.Registry = append(buildxDefaultConfig.Registry, BuildxConfigRegistry{
			ServerAddress: row.ServerAddress,
			Mirrors:       mirrors,
			EnableHttp:    row.Setting.EnableHttp,
		})
	}
	return buildxDefaultConfig, nil
}

func (self Image) GetRegistryConfig(registryUrl string) *Registry {
	if docker.Sdk.Client != nil && registryUrl == define.RegistryDefaultName {
		// 获取 docker 配置中的加速地址
		// 暂时注释掉，直接取 daemon.json 的镜像地址问题太多
		//if dockerInfo, err := docker.Sdk.Client.Info(docker.Sdk.Ctx); err == nil && dockerInfo.RegistryConfig != nil && !function.IsEmptyArray(dockerInfo.RegistryConfig.Mirrors) {
		//	result.Proxy = dockerInfo.RegistryConfig.Mirrors
		//}
	}
	result := &Registry{
		Address: make([]string, 0),
		config: registry.AuthConfig{
			ServerAddress: registryUrl,
		},
	}

	registryRow, err := dao.Registry.Where(dao.Registry.ServerAddress.Eq(registryUrl)).First()
	if err != nil || registryRow == nil || registryRow.Setting == nil {
		result.Address = append(result.Address, registryUrl)
		return result
	}

	if registryRow.Setting.EnableHttp {
		result.Address = append(result.Address, fmt.Sprintf("http://"+registryRow.ServerAddress))
	}

	if !function.IsEmptyArray(registryRow.Setting.Proxy) {
		result.Address = append(result.Address, registryRow.Setting.Proxy...)
	}

	if !function.InArray(result.Address, registryUrl) {
		result.Address = append(result.Address, registryUrl)
	}

	if registryRow.Setting != nil {
		if username, password, ok := registryRow.Setting.Auth(); ok {
			result.config.Username = username
			result.config.Password = password
		}
	}

	return result
}

func (self Image) BuildxCreateConfig(option BuildxConfig) error {
	var config bytes.Buffer
	if option.ConfigContent != nil {
		config.WriteString(*option.ConfigContent)
	} else {
		configTemplate, err := template.New("buildkitd").Funcs(template.FuncMap{
			"quote": strconv.Quote,
		}).Parse(buildxConfigTmpl)
		if err != nil {
			return err
		}
		if err := configTemplate.Execute(&config, option); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(option.ConfigPath), os.ModePerm); err != nil {
		return err
	}
	if err := os.WriteFile(option.ConfigPath, config.Bytes(), 0644); err != nil {
		return err
	}
	return nil
}
