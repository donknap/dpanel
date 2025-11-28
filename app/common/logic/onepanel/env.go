package onepanel

import (
	"fmt"

	"github.com/donknap/dpanel/common/service/compose"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/types/define"
)

var CommonEnv = map[int]docker.EnvItem{
	define.StoreEnvContainerName: {
		Name:  "CONTAINER_NAME",
		Value: compose.PlaceholderAppName,
		Rule: &docker.EnvValueRule{
			Kind: docker.EnvValueRuleRequired | docker.EnvValueTypeOnePanel,
		},
	},
	define.StoreEnvWebsiteDir: {
		Name:  "PANEL_WEBSITE_DIR",
		Value: "./wwwroot",
		Rule: &docker.EnvValueRule{
			Kind: docker.EnvValueRuleRequired | docker.EnvValueTypeOnePanel,
		},
	},
	define.StoreEnvCodeDir: {
		Name:  "CODE_DIR",
		Value: "./www",
		Rule: &docker.EnvValueRule{
			Kind: docker.EnvValueRuleRequired | docker.EnvValueTypeOnePanel,
		},
	},
	define.StoreEnvHostIp: {
		Name:  "HOST_IP",
		Value: "0.0.0.0",
		Rule: &docker.EnvValueRule{
			Kind:   docker.EnvValueTypeSelect | docker.EnvValueTypeOnePanel,
			Option: docker.NewValueItemWithArray("0.0.0.0", "127.0.0.1"),
		},
	},
	define.StoreEnvHostPort: {
		Name:  "PANEL_APP_PORT_HTTP",
		Value: "0",
		Rule: &docker.EnvValueRule{
			Kind: docker.EnvValueRuleRequired | docker.EnvValueTypeOnePanel,
		},
	},
	define.StoreEnvContainerPort: {
		Name: "APP_PORT",
		Rule: &docker.EnvValueRule{
			Kind: docker.EnvValueRuleRequired | docker.EnvValueTypeOnePanel,
		},
	},
	define.StoreEnvRunScript: {
		Name:  "EXEC_SCRIPT",
		Value: "",
		Rule: &docker.EnvValueRule{
			Kind: docker.EnvValueRuleRequired | docker.EnvValueTypeOnePanel,
		},
	},
	define.StoreEnvDBHost: {
		Name:  "PANEL_DB_HOST",
		Value: "",
		Rule: &docker.EnvValueRule{
			Kind: docker.EnvValueRuleRequired | docker.EnvValueTypeOnePanel,
		},
	},
	define.StoreEnvDBPort: {
		Name:  "PANEL_DB_PORT",
		Value: "3306",
		Rule: &docker.EnvValueRule{
			Kind: docker.EnvValueRuleRequired | docker.EnvValueTypeNumber | docker.EnvValueTypeOnePanel,
		},
	},
}

var DefaultEnv = map[string][]docker.EnvItem{
	"node": {
		CommonEnv[define.StoreEnvCodeDir],
		CommonEnv[define.StoreEnvHostIp], CommonEnv[define.StoreEnvHostPort], CommonEnv[define.StoreEnvContainerPort], CommonEnv[define.StoreEnvRunScript],
		{
			Name:  "NODE_VERSION",
			Value: compose.PlaceholderAppVersion,
			Rule: &docker.EnvValueRule{
				Kind: docker.EnvValueRuleDisabled | docker.EnvValueTypeOnePanel,
			},
		},
		{
			Name:  "CONTAINER_PACKAGE_URL",
			Value: "https://registry.npmjs.org/",
			Rule: &docker.EnvValueRule{
				Kind: docker.EnvValueTypeSelect | docker.EnvValueTypeOnePanel,
				Option: docker.NewValueItemWithArray(
					"https://registry.npmjs.org/",
					"https://registry.npmmirror.com/",
					"https://mirrors.cloud.tencent.com/npm/",
				),
			},
		},
		{
			Name:  "CUSTOM_SCRIPT",
			Value: "1",
			Rule: &docker.EnvValueRule{
				Kind: docker.EnvValueRuleDisabled | docker.EnvValueTypeOnePanel,
			},
		},
	},
	"php": {
		CommonEnv[define.StoreEnvWebsiteDir], CommonEnv[define.StoreEnvHostPort],
		{
			Name:  "IMAGE_NAME",
			Value: fmt.Sprintf("%s-%s:%s", compose.PlaceholderAppName, compose.PlaceholderAppTaskName, compose.PlaceholderAppVersion),
			Rule: &docker.EnvValueRule{
				Kind: docker.EnvValueRuleDisabled | docker.EnvValueTypeOnePanel,
			},
		},
	},
	"go": {
		CommonEnv[define.StoreEnvCodeDir],
		CommonEnv[define.StoreEnvHostIp], CommonEnv[define.StoreEnvHostPort], CommonEnv[define.StoreEnvContainerPort], CommonEnv[define.StoreEnvRunScript],
		{
			Name:  "GO_VERSION",
			Value: compose.PlaceholderAppVersion,
			Rule: &docker.EnvValueRule{
				Kind: docker.EnvValueRuleDisabled | docker.EnvValueTypeOnePanel,
			},
		},
	},
	"java": {
		CommonEnv[define.StoreEnvCodeDir],
		CommonEnv[define.StoreEnvHostIp], CommonEnv[define.StoreEnvHostPort], CommonEnv[define.StoreEnvContainerPort], CommonEnv[define.StoreEnvRunScript],
		{
			Name:  "JAVA_VERSION",
			Value: compose.PlaceholderAppVersion,
			Rule: &docker.EnvValueRule{
				Kind: docker.EnvValueRuleDisabled | docker.EnvValueTypeOnePanel,
			},
		},
	},
}
