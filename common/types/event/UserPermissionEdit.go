package event

import "github.com/donknap/dpanel/common/accessor"

var UserPermissionEditEvent = "user_permission_edit"

type UserPermissionEdit struct {
	Username      string
	Permission    accessor.DataPermissionItem
	OldPermission accessor.DataPermissionItem
}
