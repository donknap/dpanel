package baota

import (
	"github.com/donknap/dpanel/common/service/compose"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/types/define"
)

var CommonEnv = map[int]types.EnvItem{
	define.StoreEnvContainerName: {
		Name:  "CONTAINER_NAME",
		Value: compose.PlaceholderAppName,
		Rule: &types.EnvValueRule{
			Kind: types.EnvValueRuleRequired | types.EnvValueTypeBaoTa,
		},
	},
	define.StoreEnvWebsiteDir: {
		Name:  "APP_PATH",
		Value: "./",
		Rule: &types.EnvValueRule{
			Kind: types.EnvValueRuleRequired | types.EnvValueTypeBaoTa,
		},
	},
	define.StoreEnvVersion: {
		Name:  "VERSION",
		Value: compose.PlaceholderAppVersion,
		Rule: &types.EnvValueRule{
			Kind: types.EnvValueRuleDisabled | types.EnvValueTypeBaoTa,
		},
	},
	define.StoreEnvHostIp: {
		Name:  "HOST_IP",
		Value: "0.0.0.0",
		Rule: &types.EnvValueRule{
			Kind:   types.EnvValueTypeSelect | types.EnvValueTypeBaoTa,
			Option: types.NewValueItemWithArray("0.0.0.0", "127.0.0.1"),
		},
	},
	define.StoreEnvLimitCpu: {
		Name:  "CPUS",
		Value: "0",
		Rule: &types.EnvValueRule{
			Kind: types.EnvValueRuleRequired | types.EnvValueTypeBaoTa,
		},
	},
	define.StoreEnvLimitMemory: {
		Name:  "MEMORY_LIMIT",
		Value: "0",
		Rule: &types.EnvValueRule{
			Kind: types.EnvValueRuleRequired | types.EnvValueTypeBaoTa,
		},
	},
}
