package event

import "github.com/donknap/dpanel/common/accessor"

var DataPermissionAddEvent = "data_permission_add"
var DataPermissionDeleteEvent = "data_permission_delete"
var DataPermissionEditEvent = "data_permission_edit"

type DataPermissionAddPayload struct {
	Usernames      []string
	Permission     accessor.DataPermissionItem
	PermissionType string
	Append         bool
}

type DataPermissionDeletePayload struct {
	Username   string
	Permission accessor.DataPermissionItem
}

type DataPermissionEditPayload struct {
	Username      string
	Permission    accessor.DataPermissionItem
	OldPermission accessor.DataPermissionItem
}
