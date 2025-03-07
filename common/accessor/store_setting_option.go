package accessor

import "github.com/donknap/dpanel/common/service/docker"

var (
	StoreTypeOnePanel      = "1panel"
	StoreTypeOnePanelLocal = "1panel-local"
	StoreTypePortainer     = "portainer"
	StoreTypeCasaOs        = "casaos"
)

type StoreAppItem struct {
	Title       string                         `json:"title"`
	Name        string                         `json:"name"`
	Logo        string                         `json:"logo"`
	Content     string                         `json:"content"`
	Description string                         `json:"description"`
	Tag         []string                       `json:"tag"`
	Website     string                         `json:"website"`
	Version     map[string]StoreAppVersionItem `json:"version"`
}

type StoreAppVersionScriptItem struct {
	Install   string `json:"install,omitempty"`
	Uninstall string `json:"uninstall,omitempty"`
	Upgrade   string `json:"upgrade,omitempty"`
}

type StoreAppVersionItem struct {
	Name        string                     `json:"name"`
	ComposeFile string                     `json:"composeFile" yaml:"composeFile"`
	Environment []docker.EnvItem           `json:"environment,omitempty"`
	Script      *StoreAppVersionScriptItem `json:"script,omitempty"`
}

type StoreSettingOption struct {
	Type      string         `json:"type,omitempty"`
	Url       string         `json:"url,omitempty"`
	Apps      []StoreAppItem `json:"apps,omitempty"`
	UpdatedAt int64          `json:"updatedAt,omitempty"`
}
