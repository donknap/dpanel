package common

import "github.com/donknap/dpanel/common/service/docker"

type EnvListResult struct {
	CurrentName string           `json:"currentName"`
	List        []*docker.Client `json:"list"`
}
