package context

import (
	"bytes"
	"strconv"
	"strings"
	"text/template"

	dockerregistry "github.com/docker/docker/registry"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/function"
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

type buildxConfig struct {
	WorkerNetworkMode string
	Registry          []buildxConfigRegistry
}

type buildxConfigRegistry struct {
	ServerAddress string
	Mirrors       []string
	EnableHttp    bool
}

func (self *contextService) defaultConfig() (string, error) {
	result := buildxConfig{
		WorkerNetworkMode: "host",
		Registry:          make([]buildxConfigRegistry, 0),
	}
	registryRows, err := dao.Registry.Order(dao.Registry.ServerAddress.Asc()).Find()
	if err != nil {
		return "", err
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
			if mirrorHost == dockerregistry.DefaultNamespace || mirrorHost == dockerregistry.IndexHostname || mirrorHost == dockerregistry.DefaultRegistryHost {
				continue
			}
			mirrors = append(mirrors, mirror)
		}
		if function.IsEmptyArray(mirrors) && !row.Setting.EnableHttp {
			continue
		}
		result.Registry = append(result.Registry, buildxConfigRegistry{
			ServerAddress: row.ServerAddress,
			Mirrors:       mirrors,
			EnableHttp:    row.Setting.EnableHttp,
		})
	}
	var config bytes.Buffer
	configTemplate, err := template.New("buildkitd").Funcs(template.FuncMap{
		"quote": strconv.Quote,
	}).Parse(buildxConfigTmpl)
	if err != nil {
		return "", err
	}
	if err := configTemplate.Execute(&config, result); err != nil {
		return "", err
	}
	return config.String(), nil
}
