package logic

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
)

const buildxConfigTmpl = `
{{- if .WorkerNetworkMode }}
[worker.oci]
  networkMode = {{ quote .WorkerNetworkMode }}

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

type ImageBuildx struct{}

type BuildxConfig struct {
	ConfigPath        string
	ConfigContent     *string
	WorkerNetworkMode string
	Registry          []BuildxConfigRegistry
}

type BuildxConfigRegistry struct {
	ServerAddress string
	Mirrors       []string
	EnableHttp    bool
}

func (ImageBuildx) ResolveConfig(dockerEnvName string) (BuildxConfig, error) {
	result := BuildxConfig{
		ConfigPath:        filepath.Join(storage.Local{}.GetStorageLocalPath(), "buildx", dockerEnvName, "config.toml"),
		WorkerNetworkMode: "host",
		Registry:          make([]BuildxConfigRegistry, 0),
	}
	registryRows, err := dao.Registry.Order(dao.Registry.ServerAddress.Asc()).Find()
	if err != nil {
		return result, err
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
		result.Registry = append(result.Registry, BuildxConfigRegistry{
			ServerAddress: row.ServerAddress,
			Mirrors:       mirrors,
			EnableHttp:    row.Setting.EnableHttp,
		})
	}
	return result, nil
}

func (ImageBuildx) WriteConfig(option BuildxConfig) error {
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
	return os.WriteFile(option.ConfigPath, config.Bytes(), 0644)
}
