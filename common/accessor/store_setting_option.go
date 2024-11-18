package accessor

var (
	StoreType1Panel    = "1panel"
	StoreTypePortainer = "portainer"
)

type StoreSettingOption struct {
	Type string `json:"type,omitempty"`
	Git  string `json:"git,omitempty"`
	Url  string `json:"url,omitempty"`
}
