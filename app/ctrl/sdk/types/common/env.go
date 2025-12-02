package common

import (
	"github.com/donknap/dpanel/common/service/docker/types"
)

type EnvListResult struct {
	CurrentName string             `json:"currentName"`
	List        []*types.DockerEnv `json:"list"`
}
