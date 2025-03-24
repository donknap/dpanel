package event

import "github.com/donknap/dpanel/common/accessor"

var UserPermissionDelEvent = "user_permission_del"

type UserPermissionDel struct {
	Username   string
	Permission accessor.DataPermissionItem
}
