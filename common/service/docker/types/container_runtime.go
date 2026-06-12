package types

import "time"

type ContainerRuntime struct {
	ContainerRuntimeEvent
	ContainerID   string                  `json:"containerId"`
	ContainerName string                  `json:"containerName"`
	History       []ContainerRuntimeEvent `json:"history"`
}

func (self ContainerRuntime) ActionCount(actions ...string) int {
	if len(actions) == 0 {
		return 0
	}
	actionMap := make(map[string]struct{}, len(actions))
	for _, action := range actions {
		actionMap[action] = struct{}{}
	}
	count := 0
	for _, item := range self.History {
		if _, ok := actionMap[item.Action]; ok {
			count += 1
		}
	}
	return count
}

type ContainerRuntimeEvent struct {
	Action    string    `json:"action"`
	State     string    `json:"state"`
	Status    string    `json:"status"`
	Running   bool      `json:"running"`
	ExitCode  int       `json:"exitCode,omitempty"`
	OOMKilled bool      `json:"oomKilled,omitempty"`
	Time      time.Time `json:"time"`
}
