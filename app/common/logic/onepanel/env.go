package onepanel

import (
	"fmt"
	"github.com/donknap/dpanel/common/service/compose"
	"github.com/donknap/dpanel/common/service/docker"
)

const (
	_ = iota
	ContainerName
	imageName
	websiteDir
	codeDir
	hostIp
	hostPort
	containerPort
	runScript
)

var CommonEnv = map[int]docker.EnvItem{
	ContainerName: {
		Name:  "CONTAINER_NAME",
		Value: compose.PlaceholderAppName,
		Rule: &docker.EnvValueRule{
			Kind: docker.EnvValueRuleRequired | docker.EnvValueTypeOnePanel,
		},
	},
	websiteDir: {
		Name:  "PANEL_WEBSITE_DIR",
		Value: fmt.Sprintf("%s/%s", compose.PlaceholderWebsitePath, compose.PlaceholderAppTaskName),
		Rule: &docker.EnvValueRule{
			Kind: docker.EnvValueRuleRequired | docker.EnvValueTypeOnePanel,
		},
	},
	codeDir: {
		Name:  "CODE_DIR",
		Value: "./www",
		Rule: &docker.EnvValueRule{
			Kind: docker.EnvValueRuleRequired | docker.EnvValueTypeOnePanel,
		},
	},
	hostIp: {
		Name:  "HOST_IP",
		Value: "0.0.0.0",
		Rule: &docker.EnvValueRule{
			Kind:   docker.EnvValueTypeSelect | docker.EnvValueTypeOnePanel,
			Option: docker.NewValueItemWithArray("0.0.0.0", "127.0.0.1"),
		},
	},
	hostPort: {
		Name:  "PANEL_APP_PORT_HTTP",
		Value: "0",
		Rule: &docker.EnvValueRule{
			Kind: docker.EnvValueRuleRequired | docker.EnvValueTypeOnePanel,
		},
	},
	containerPort: {
		Name: "APP_PORT",
		Rule: &docker.EnvValueRule{
			Kind: docker.EnvValueRuleRequired | docker.EnvValueTypeOnePanel,
		},
	},
	runScript: {
		Name:  "EXEC_SCRIPT",
		Value: "",
		Rule: &docker.EnvValueRule{
			Kind: docker.EnvValueRuleRequired | docker.EnvValueTypeOnePanel,
		},
	},
}

var DefaultEnv = map[string][]docker.EnvItem{
	"node": {
		CommonEnv[codeDir],
		CommonEnv[hostIp], CommonEnv[hostPort], CommonEnv[containerPort], CommonEnv[runScript],
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
		CommonEnv[websiteDir], CommonEnv[hostPort],
		{
			Name:  "IMAGE_NAME",
			Value: fmt.Sprintf("%s-%s:%s", compose.PlaceholderAppName, compose.PlaceholderAppTaskName, compose.PlaceholderAppVersion),
			Rule: &docker.EnvValueRule{
				Kind: docker.EnvValueRuleDisabled | docker.EnvValueTypeOnePanel,
			},
		},
	},
	"go": {
		CommonEnv[codeDir],
		CommonEnv[hostIp], CommonEnv[hostPort], CommonEnv[containerPort], CommonEnv[runScript],
		{
			Name:  "GO_VERSION",
			Value: compose.PlaceholderAppVersion,
			Rule: &docker.EnvValueRule{
				Kind: docker.EnvValueRuleDisabled | docker.EnvValueTypeOnePanel,
			},
		},
	},
	"java": {
		CommonEnv[codeDir],
		CommonEnv[hostIp], CommonEnv[hostPort], CommonEnv[containerPort], CommonEnv[runScript],
		{
			Name:  "JAVA_VERSION",
			Value: compose.PlaceholderAppVersion,
			Rule: &docker.EnvValueRule{
				Kind: docker.EnvValueRuleDisabled | docker.EnvValueTypeOnePanel,
			},
		},
	},
}
