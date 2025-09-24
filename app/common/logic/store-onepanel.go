package logic

import (
	"fmt"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/service/compose"
	"github.com/donknap/dpanel/common/service/docker"
)

func (self Store) getOnePanelYamlEnv(storeVersionItem accessor.StoreAppVersionItem) map[string]docker.EnvItem {
	return map[string]docker.EnvItem{
		"${CONTAINER_NAME}": {
			Name:  "CONTAINER_NAME",
			Label: "容器名称",
			Labels: map[string]string{
				"zh": "容器名称",
				"en": "Container Name",
			},
			Rule: &docker.ValueRuleItem{
				Kind: docker.EnvValueRuleRequired,
			},
			Value: compose.PlaceholderProjectName,
		},
		"${IMAGE_NAME}": {
			Name:  "IMAGE_NAME",
			Label: "镜像名称",
			Labels: map[string]string{
				"zh": "镜像名称",
				"en": "Image Name",
			},
			Value: fmt.Sprintf("php-%s:%s", compose.PlaceholderProjectName, storeVersionItem.Name),
		},
		"${PANEL_WEBSITE_DIR}": {
			Name:  "PANEL_WEBSITE_DIR",
			Label: "网站目录",
			Labels: map[string]string{
				"zh": "网站目录",
				"en": "Website Root",
			},
			Value: fmt.Sprintf("%s/%s", compose.PlaceholderWebsiteDefaultPath, compose.PlaceholderProjectName),
		},
		"${CODE_DIR}": {
			Name:  "CODE_DIR",
			Label: "网站目录",
			Labels: map[string]string{
				"zh": "网站目录",
				"en": "Website Root",
			},
			Value: fmt.Sprintf("%s/%s", compose.PlaceholderWebsiteDefaultPath, compose.PlaceholderProjectName),
		},
		"${NODE_VERSION}": {
			Name:  "NODE_VERSION",
			Label: "Node 版本",
			Labels: map[string]string{
				"zh": "Node 版本",
				"en": "Node Version",
			},
			Value: storeVersionItem.Name,
			Rule: &docker.ValueRuleItem{
				Kind: docker.EnvValueRuleDisabled,
			},
		},
		"${HOST_IP}": {
			Name:  "HOST_IP",
			Label: "映射 IP",
			Labels: map[string]string{
				"zh": "主机映射 IP",
			},
			Value: "0.0.0.0",
		},
		"${PANEL_APP_PORT_HTTP}": {
			Name:  "PANEL_APP_PORT_HTTP",
			Label: "主机映射端口",
			Labels: map[string]string{
				"zh": "网站端口",
			},
			Value: "0",
		},
		"${APP_PORT}": {
			Name:  "APP_PORT",
			Label: "容器内端口",
			Labels: map[string]string{
				"zh": "容器内端口",
			},
			Rule: &docker.ValueRuleItem{
				Kind: docker.EnvValueRuleRequired,
			},
		},
	}
}

func (self Store) appendOnePanelEnv() []docker.EnvItem {
	return []docker.EnvItem{
		{
			Name:  "CONTAINER_PACKAGE_URL",
			Label: "镜像源",
			Labels: map[string]string{
				"zh": "镜像源",
				"en": "Npm Registry",
			},
			Value: "https://registry.npmjs.org/",
			Rule: &docker.ValueRuleItem{
				Kind: docker.EnvValueTypeSelect,
				Option: []docker.ValueItem{
					{
						Name:  "https://registry.npmjs.org/",
						Value: "https://registry.npmjs.org/",
					},
					{
						Name:  "https://registry.npmmirror.com",
						Value: "https://registry.npmmirror.com",
					},
					{
						Name:  "https://mirrors.cloud.tencent.com/npm/",
						Value: "https://mirrors.cloud.tencent.com/npm/",
					},
				},
			},
		},
		{
			Name:  "CUSTOM_SCRIPT",
			Label: "自定义启动命令",
			Labels: map[string]string{
				"zh": "自定义启动命令",
				"en": "Custom Script",
			},
			Value: "Yes",
			Rule: &docker.ValueRuleItem{
				Kind: docker.EnvValueRuleDisabled,
			},
		},
		{
			Name:  "EXEC_SCRIPT",
			Label: "启动命令",
			Labels: map[string]string{
				"zh": "启动命令",
				"en": "Script",
			},
		},
	}
}
