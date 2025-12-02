package onepanel

import (
	"fmt"

	"github.com/donknap/dpanel/common/service/compose"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/types/define"
)

var CommonEnv = map[int]types.EnvItem{
	define.StoreEnvContainerName: {
		Name:  "CONTAINER_NAME",
		Value: compose.PlaceholderAppName,
		Rule: &types.EnvValueRule{
			Kind: types.EnvValueRuleRequired | types.EnvValueTypeOnePanel,
		},
	},
	define.StoreEnvWebsiteDir: {
		Name:  "PANEL_WEBSITE_DIR",
		Value: "./wwwroot",
		Rule: &types.EnvValueRule{
			Kind: types.EnvValueRuleRequired | types.EnvValueTypeOnePanel,
		},
	},
	define.StoreEnvCodeDir: {
		Name:  "CODE_DIR",
		Value: "./www",
		Rule: &types.EnvValueRule{
			Kind: types.EnvValueRuleRequired | types.EnvValueTypeOnePanel,
		},
	},
	define.StoreEnvHostIp: {
		Name:  "HOST_IP",
		Value: "0.0.0.0",
		Rule: &types.EnvValueRule{
			Kind:   types.EnvValueTypeSelect | types.EnvValueTypeOnePanel,
			Option: types.NewValueItemWithArray("0.0.0.0", "127.0.0.1"),
		},
	},
	define.StoreEnvHostPort: {
		Name:  "PANEL_APP_PORT_HTTP",
		Value: "0",
		Rule: &types.EnvValueRule{
			Kind: types.EnvValueRuleRequired | types.EnvValueTypeOnePanel,
		},
	},
	define.StoreEnvContainerPort: {
		Name: "APP_PORT",
		Rule: &types.EnvValueRule{
			Kind: types.EnvValueRuleRequired | types.EnvValueTypeOnePanel,
		},
	},
	define.StoreEnvRunScript: {
		Name:  "EXEC_SCRIPT",
		Value: "",
		Rule: &types.EnvValueRule{
			Kind: types.EnvValueRuleRequired | types.EnvValueTypeOnePanel,
		},
	},
	define.StoreEnvDBHost: {
		Name:  "PANEL_DB_HOST",
		Value: "",
		Rule: &types.EnvValueRule{
			Kind: types.EnvValueRuleRequired | types.EnvValueTypeOnePanel,
		},
	},
	define.StoreEnvDBPort: {
		Name:  "PANEL_DB_PORT",
		Value: "3306",
		Rule: &types.EnvValueRule{
			Kind: types.EnvValueRuleRequired | types.EnvValueTypeNumber | types.EnvValueTypeOnePanel,
		},
	},
}

var DefaultEnv = map[string][]types.EnvItem{
	"node": {
		CommonEnv[define.StoreEnvCodeDir],
		CommonEnv[define.StoreEnvHostIp], CommonEnv[define.StoreEnvHostPort], CommonEnv[define.StoreEnvContainerPort], CommonEnv[define.StoreEnvRunScript],
		{
			Name:  "NODE_VERSION",
			Value: compose.PlaceholderAppVersion,
			Rule: &types.EnvValueRule{
				Kind: types.EnvValueRuleDisabled | types.EnvValueTypeOnePanel,
			},
		},
		{
			Name:  "CONTAINER_PACKAGE_URL",
			Value: "https://registry.npmjs.org/",
			Rule: &types.EnvValueRule{
				Kind: types.EnvValueTypeSelect | types.EnvValueTypeOnePanel,
				Option: types.NewValueItemWithArray(
					"https://registry.npmjs.org/",
					"https://registry.npmmirror.com/",
					"https://mirrors.cloud.tencent.com/npm/",
				),
			},
		},
		{
			Name:  "CUSTOM_SCRIPT",
			Value: "1",
			Rule: &types.EnvValueRule{
				Kind: types.EnvValueRuleDisabled | types.EnvValueTypeOnePanel,
			},
		},
	},
	"php": {
		CommonEnv[define.StoreEnvWebsiteDir], CommonEnv[define.StoreEnvHostPort],
		{
			Name:  "IMAGE_NAME",
			Value: fmt.Sprintf("%s-%s:%s", compose.PlaceholderAppName, compose.PlaceholderAppTaskName, compose.PlaceholderAppVersion),
			Rule: &types.EnvValueRule{
				Kind: types.EnvValueRuleDisabled | types.EnvValueTypeOnePanel,
			},
		},
	},
	"go": {
		CommonEnv[define.StoreEnvCodeDir],
		CommonEnv[define.StoreEnvHostIp], CommonEnv[define.StoreEnvHostPort], CommonEnv[define.StoreEnvContainerPort], CommonEnv[define.StoreEnvRunScript],
		{
			Name:  "GO_VERSION",
			Value: compose.PlaceholderAppVersion,
			Rule: &types.EnvValueRule{
				Kind: types.EnvValueRuleDisabled | types.EnvValueTypeOnePanel,
			},
		},
	},
	"java": {
		CommonEnv[define.StoreEnvCodeDir],
		CommonEnv[define.StoreEnvHostIp], CommonEnv[define.StoreEnvHostPort], CommonEnv[define.StoreEnvContainerPort], CommonEnv[define.StoreEnvRunScript],
		{
			Name:  "JAVA_VERSION",
			Value: compose.PlaceholderAppVersion,
			Rule: &types.EnvValueRule{
				Kind: types.EnvValueRuleDisabled | types.EnvValueTypeOnePanel,
			},
		},
	},
}
