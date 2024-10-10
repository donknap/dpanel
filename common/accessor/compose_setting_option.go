package accessor

import (
	"fmt"
)

type ComposeSettingOption struct {
	RawYaml     string    `json:"rawYaml,omitempty"`
	Environment []EnvItem `json:"environment"`
	Status      string    `json:"status"`
	Type        string    `json:"type"`
	Uri         string    `json:"uri,omitempty"`
}

func (self ComposeSettingOption) GetEnvList() []string {
	result := make([]string, 0)
	for _, item := range self.Environment {
		result = append(result, fmt.Sprintf("%s=%s", item.Name, item.Value))
	}
	return result
}
