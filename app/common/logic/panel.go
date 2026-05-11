package logic

import (
	"bytes"
	"fmt"
	"html/template"
	"path/filepath"

	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/storage"
)

type Panel struct {
}

const panelUpdateCommandTemplate = `
{{- if eq .type "container" -}}
docker run --rm \
-v /var/run/docker.sock:/var/run/docker.sock \
{{ .installerImage }} \
upgrade -y -d \
--name {{ .name }}{{ if .version }} \
--version {{ .version }}{{ end }}{{ if .edition }} \
--edition {{ .edition }}{{ end }}{{ if .enableDev }} \
--dev{{ end }}{{ if .dns }} \
--dns {{ .dns }}{{ end }}{{ if .proxy }} \
--proxy {{ .proxy }}{{ end }}{{ if .enableBackup }} \
--backup{{ end }}
{{- else -}}
curl -sSL https://dpanel.cc/quick.sh | bash -s -- \
upgrade -y -d \
--name {{ .name }} \
{{ if .version }} \
--version {{ .version }}{{ end }}{{ if .edition }} \
--edition {{ .edition }}{{ end }}{{ if .enableDev }} \
--dev{{ end }}{{ if .dns }} \
--dns {{ .dns }}{{ end }}{{ if .proxy }} \
--proxy {{ .proxy }}{{ end }}{{ if .enableBackup }} \
--backup{{ end }}
{{- end }}
`

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
	commandTemplate, err := template.New("panel-update-command").Parse(panelUpdateCommandTemplate)
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
