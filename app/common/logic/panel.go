package logic

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"path/filepath"
	"regexp"

	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/storage"
)

type Panel struct {
}

var installerArgKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]*$`)

// 容器内触发升级时必须以 detached 模式运行 installer，避免当前进程/容器退出后升级中断。
const panelUpdateCommandTemplate = `
{{- if eq .type "container" -}}
docker run -d --rm -v /var/run/docker.sock:/var/run/docker.sock {{ .installerDownloadSource }} upgrade -y --name {{ shellSafe .name }}{{- range $key, $value := .params }}{{- $arg := shellSafe $value }}{{- if ne $arg "" }} --{{ $key }} {{ $arg }}{{- end }}{{- end }}
{{- else -}}
curl -sSL https://dpanel.cc/quick.sh | bash -s -- upgrade -y -d --name {{ shellSafe .name }}{{- range $key, $value := .params }}{{- $arg := shellSafe $value }}{{- if ne $arg "" }} --{{ $key }} {{ $arg }}{{- end }}{{- end }}
{{- end }}`

func (self Panel) GetPanelPath() []*types.ValueItem {
	savePath := []*types.ValueItem{
		{Name: "database", Value: "./dpanel.db"},
		{Name: "acme", Value: "./acme"},
		{Name: "backup", Value: "./backup"},
		{Name: "cert", Value: "./cert"},
		{Name: "compose-local", Value: "./compose"},
		{Name: "nginx", Value: "./nginx"},
		{Name: "script", Value: "./script"},
		{Name: "export/file", Value: "./storage/export/file"},
		{Name: "export/container", Value: "./storage/export/container"},
		{Name: "export/image", Value: "./storage/export/image"},
		{Name: "temp", Value: "./storage/temp"},
		{Name: "image", Value: "./storage/image"},
		{Name: "store", Value: "./store"},
		{Name: "lic", Value: "./dpanel.lic"},
	}

	if setting, err := (Setting{}).GetValue(SettingGroupSetting, SettingGroupSettingDocker); err == nil {
		for _, item := range setting.Value.Docker {
			if item.EnableComposePath {
				name := fmt.Sprintf("compose-%s", item.Name)
				savePath = append(savePath, &types.ValueItem{
					Name:  name,
					Value: name,
				})
			}
		}
	}

	return savePath
}

func (self Panel) SaveRootPath() string {
	return filepath.Join(storage.Local{}.GetBackupPath(), "dpanel")
}

func (self Panel) MakeUpdateCommand(params map[string]any) (string, error) {
	if raw, exists := params["params"]; exists && raw != nil {
		args, ok := raw.(map[string]any)
		if !ok {
			return "", errors.New("invalid update params")
		}
		for key := range args {
			if !installerArgKeyPattern.MatchString(key) {
				return "", fmt.Errorf("invalid update param key: %s", key)
			}
		}
	}

	commandTemplate, err := template.New("panel-update-command").Funcs(template.FuncMap{
		"shellSafe": function.SafeShell,
	}).Parse(panelUpdateCommandTemplate)
	if err != nil {
		return "", err
	}

	commandBuffer := new(bytes.Buffer)
	err = commandTemplate.Execute(commandBuffer, params)
	if err != nil {
		return "", err
	}
	return commandBuffer.String(), nil
}
