package event

import "github.com/donknap/dpanel/common/accessor"

var UserPermissionAddEvent = "user_permission_add"

type UserPermissionAdd struct {
	Usernames      []string
	Permission     accessor.DataPermissionItem
	PermissionType string
	Append         bool
}
