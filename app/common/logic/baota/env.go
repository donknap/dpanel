package baota

import (
	"github.com/donknap/dpanel/common/service/compose"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/types/define"
)

var CommonEnv = map[int]docker.EnvItem{
	define.StoreEnvContainerName: {
		Name:  "CONTAINER_NAME",
		Value: compose.PlaceholderAppName,
		Rule: &docker.EnvValueRule{
			Kind: docker.EnvValueRuleRequired | docker.EnvValueTypeBaoTa,
		},
	},
	define.StoreEnvWebsiteDir: {
		Name:  "APP_PATH",
		Value: "./",
		Rule: &docker.EnvValueRule{
			Kind: docker.EnvValueRuleRequired | docker.EnvValueTypeBaoTa,
		},
	},
	define.StoreEnvVersion: {
		Name:  "VERSION",
		Value: compose.PlaceholderAppVersion,
		Rule: &docker.EnvValueRule{
			Kind: docker.EnvValueRuleDisabled | docker.EnvValueTypeBaoTa,
		},
	},
	define.StoreEnvHostIp: {
		Name:  "HOST_IP",
		Value: "0.0.0.0",
		Rule: &docker.EnvValueRule{
			Kind:   docker.EnvValueTypeSelect | docker.EnvValueTypeBaoTa,
			Option: docker.NewValueItemWithArray("0.0.0.0", "127.0.0.1"),
		},
	},
	define.StoreEnvLimitCpu: {
		Name:  "CPUS",
		Value: "0",
		Rule: &docker.EnvValueRule{
			Kind: docker.EnvValueRuleRequired | docker.EnvValueTypeBaoTa,
		},
	},
	define.StoreEnvLimitMemory: {
		Name:  "MEMORY_LIMIT",
		Value: "0",
		Rule: &docker.EnvValueRule{
			Kind: docker.EnvValueRuleRequired | docker.EnvValueTypeBaoTa,
		},
	},
}
