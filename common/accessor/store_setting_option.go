package accessor

import "github.com/donknap/dpanel/common/service/docker"

var (
	StoreTypeOnePanel      = "1panel"
	StoreTypeOnePanelLocal = "1panel-local"
	StoreTypePortainer     = "portainer"
	StoreTypeCasaOs        = "casaos"
	StoreTypeBaota         = "baota"
)

type StoreAppItem struct {
	Title        string                         `json:"title"`
	Name         string                         `json:"name"`
	Logo         string                         `json:"logo"`
	Content      string                         `json:"content"` // Deprecated: instead Contents["zh"]
	Contents     map[string]string              `json:"contents,omitempty"`
	Description  string                         `json:"description"` // Deprecated: instead Descriptions["zh"]
	Descriptions map[string]string              `json:"descriptions,omitempty"`
	Tag          []string                       `json:"tag"`
	Website      string                         `json:"website"`
	Version      map[string]StoreAppVersionItem `json:"version"`
}

type StoreAppVersionItem struct {
	Name        string                   `json:"name"`
	ComposeFile string                   `json:"composeFile" yaml:"composeFile"`
	Environment []docker.EnvItem         `json:"environment,omitempty"`
	Script      map[string]string        `json:"script,omitempty"`
	Depend      *StoreAppVersionTaskItem `json:"task,omitempty" yaml:"task,omitempty"`
}

type StoreAppVersionTaskItem struct {
	Name             string           `json:"name"`
	Environment      []docker.EnvItem `json:"environment,omitempty"`
	BuildComposeFile string           `json:"buildComposeFile,omitempty" yaml:"buildComposeFile"`
}

type StoreSettingOption struct {
	Type      string         `json:"type,omitempty"`
	Url       string         `json:"url,omitempty"`
	Apps      []StoreAppItem `json:"apps,omitempty"`
	UpdatedAt int64          `json:"updatedAt,omitempty"`
}
