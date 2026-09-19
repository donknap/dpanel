package types

import (
	"os"
	"time"
)

type FileData struct {
	Name     string      `json:"name"`
	Size     int64       `json:"size"`
	Mode     os.FileMode `json:"mode"`
	ModeText string      `json:"modeText"`
	ModTime  time.Time   `json:"modTime"`
	UID      uint32      `json:"uid"`
	GID      uint32      `json:"gid"`
	User     string      `json:"user"`
	Group    string      `json:"group"`
	LinkName string      `json:"linkName"`
}

type SizeData struct {
	Size int64 `json:"size"`
}

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

type FilesystemUsage struct {
	Used           uint64  `json:"used"`
	Available      uint64  `json:"available"`
	Total          uint64  `json:"total"`
	InodeUsed      *uint64 `json:"inodeUsed,omitempty"`
	InodeAvailable *uint64 `json:"inodeAvailable,omitempty"`
	InodeTotal     *uint64 `json:"inodeTotal,omitempty"`
}
