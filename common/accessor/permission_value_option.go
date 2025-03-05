package accessor

type PermissionItemSetting struct {
	Key     string                 `json:"key,omitempty"`
	Value   interface{}            `json:"value,omitempty"`
	Options map[string]interface{} `json:"options,omitempty"`
}

type MenuPermission struct {
	Permissions map[string]struct{}    `json:"permissions,omitempty"`
	Uris        map[string]struct{}    `json:"uris,omitempty"`
	Options     map[string]interface{} `json:"options,omitempty"`
}

type DataPermission struct {
	Permissions map[string]PermissionItemSetting `json:"permissions,omitempty"`
}

type PermissionValueOption struct {
	MenuPermission *MenuPermission `json:"menuPermission,omitempty"`
	DataPermission *DataPermission `json:"dataPermission,omitempty"`
}
