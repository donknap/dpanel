package accessor

var (
	StoreTypeOnePanel  = "1panel"
	StoreTypePortainer = "portainer"
	StoreTypeCasaOs    = "casaos"
)

type StoreAppItem struct {
	Name        string                         `json:"name"`
	Logo        string                         `json:"logo"`
	Description string                         `json:"description"`
	Tag         []string                       `json:"tag"`
	Version     map[string]StoreAppVersionItem `json:"version"`
}

type StoreAppVersionItem struct {
	Name        string    `json:"name"`
	File        string    `json:"file"`
	Environment []EnvItem `json:"environment"`
}

type StoreSettingOption struct {
	Type      string         `json:"type,omitempty"`
	Url       string         `json:"url,omitempty"`
	Apps      []StoreAppItem `json:"apps,omitempty"`
	UpdatedAt int64          `json:"updatedAt,omitempty"`
}
