package logic

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"text/template"
	"time"

	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/storage"
	types2 "github.com/donknap/dpanel/common/types"
	"github.com/donknap/dpanel/common/types/define"
)

type Panel struct {
}

var installerArgKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]*$`)

// 容器内触发升级时必须以 detached 模式运行 installer，避免当前进程/容器退出后升级中断。
const panelUpdateCommandTemplate = `
{{- if eq .type "container" -}}
docker run -d --rm --pull always -v /var/run/docker.sock:/var/run/docker.sock -v {{ shellSafe .mountHost }}:/dpanel {{ .installerDownloadSource }} upgrade -y --log-path {{ shellSafe .installerLogPath }} --name {{ shellSafe .name }}{{- range $key, $value := .params }}{{- $arg := shellSafe $value }}{{- if ne $arg "" }} --{{ $key }} {{ $arg }}{{- end }}{{- end }}
{{- else -}}
curl -sSL https://dpanel.cc/quick.sh | bash -s -- upgrade -y -d --log-path {{ shellSafe .installerLogPath }} --name {{ shellSafe .name }}{{- range $key, $value := .params }}{{- $arg := shellSafe $value }}{{- if ne $arg "" }} --{{ $key }} {{ $arg }}{{- end }}{{- end }}
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

func (self Panel) ValidateProxy(proxy string) error {
	proxyUrl, err := url.ParseRequestURI(proxy)
	if err != nil || proxyUrl.Scheme == "" || proxyUrl.Host == "" || proxyUrl.Hostname() == "" {
		return errors.New("invalid proxy url")
	}
	switch proxyUrl.Scheme {
	case "http", "https", "socks5":
	default:
		return errors.New("unsupported proxy scheme")
	}

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyUrl),
		},
		Timeout: 8 * time.Second,
	}
	resp, err := client.Get("https://registry-1.docker.io/v2/")
	if err != nil {
		return errors.New("proxy external access check failed: " + err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized {
		return nil
	}
	if resp.StatusCode == http.StatusProxyAuthRequired {
		return errors.New("proxy authentication failed")
	}
	return fmt.Errorf("proxy external access check failed: %d", resp.StatusCode)
}

func (self Panel) SaveRootPath() string {
	return filepath.Join(storage.Local{}.GetBackupPath(), "dpanel")
}

func (self Panel) MakeUpdateCommand(params map[string]any) (string, error) {
	args := map[string]any{}
	if raw, exists := params["params"]; exists && raw != nil {
		var ok bool
		args, ok = raw.(map[string]any)
		if !ok {
			return "", errors.New("invalid update params")
		}
		for key := range args {
			if !installerArgKeyPattern.MatchString(key) {
				return "", fmt.Errorf("invalid update param key: %s", key)
			}
		}
	} else {
		params["params"] = args
	}
	dpanelInfo := (Setting{}).GetDPanelInfo()
	storagePath := dpanelInfo.Mount.Host
	if storagePath == "" {
		return "", errors.New("dpanel storage path not found")
	}

	if dpanelInfo.RunIn != types2.DPanelRunInContainer {
		if _, exists := args["data-path"]; !exists {
			executablePath, err := os.Executable()
			if err != nil {
				return "", err
			}
			args["data-path"] = filepath.Dir(executablePath)
		}
	}

	templateParams := make(map[string]any, len(params)+3)
	for key, value := range params {
		templateParams[key] = value
	}

	logFileName := fmt.Sprintf("upgrade-%s.log", time.Now().Format(define.DateYmdHis))
	templateParams["installerLogPath"] = filepath.Join(storagePath, "logs", logFileName)
	templateParams["type"] = dpanelInfo.RunIn
	if dpanelInfo.RunIn == types2.DPanelRunInContainer {
		templateParams["mountHost"] = storagePath
		templateParams["installerLogPath"] = filepath.Join("/dpanel", "logs", logFileName)
	}

	commandTemplate, err := template.New("panel-update-command").Funcs(template.FuncMap{
		"shellSafe": function.SafeShell,
	}).Parse(panelUpdateCommandTemplate)
	if err != nil {
		return "", err
	}

	commandBuffer := new(bytes.Buffer)
	err = commandTemplate.Execute(commandBuffer, templateParams)
	if err != nil {
		return "", err
	}
	return commandBuffer.String(), nil
}
