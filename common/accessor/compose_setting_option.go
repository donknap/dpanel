package accessor

import "fmt"

type ComposeSettingOption struct {
	Environment []EnvItem                `json:"environment,omitempty"`
	Status      string                   `json:"status,omitempty"`
	Type        string                   `json:"type"`
	Uri         []string                 `json:"uri,omitempty"`
	Override    map[string]SiteEnvOption `json:"override,omitempty"`
}

func (self ComposeSettingOption) EnvironmentToMappingWithEquals() []string {
	envList := make([]string, 0)
	for _, item := range self.Environment {
		envList = append(envList, fmt.Sprintf("%s=%s", item.Name, item.Value))
	}
	return envList
}
