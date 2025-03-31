package accessor

type DataPermissionItem struct {
	Name    string        `json:"name"`
	Uri     string        `json:"uri" binding:"required"`
	Key     string        `json:"key,omitempty" binding:"required"`
	Value   []interface{} `json:"value,omitempty" binding:"required"`
	ShowKey string        `json:"showKey,omitempty"`
}

type ResourcePermissionItem struct {
	Name    string `json:"name" binding:"required"`
	Uri     string `json:"uri" binding:"required"`
	Limit   int64  `json:"limit" binding:"required"`
	Created int64  `json:"created"`
}

type MenuPermission struct {
	Permissions map[string]struct{} `json:"permissions,omitempty"`
	Uris        map[string]struct{} `json:"uris,omitempty"`
}

type DataPermission struct {
	Permissions map[string]DataPermissionItem `json:"permissions,omitempty"`
}

type ResourcesPermission struct {
	Permissions map[string]ResourcePermissionItem `json:"permissions,omitempty"`
}

type PermissionValueOption struct {
	MenuPermission      *MenuPermission      `json:"menuPermission,omitempty"`
	DataPermission      *DataPermission      `json:"dataPermission,omitempty"`
	ResourcesPermission *ResourcesPermission `json:"resources,omitempty"`
}
