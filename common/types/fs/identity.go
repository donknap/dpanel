package fs

type UserIdentity struct {
	Name        string `json:"name"`
	UID         uint32 `json:"uid"`
	GID         uint32 `json:"gid"`
	Description string `json:"description"`
}

type GroupIdentity struct {
	Name string `json:"name"`
	GID  uint32 `json:"gid"`
}

type IdentityList struct {
	User  []UserIdentity  `json:"user"`
	Group []GroupIdentity `json:"group"`
}
