package fs

import (
	"os"
	"strconv"
	"strings"
)

type userIdentity struct {
	Name        string `json:"name"`
	UID         uint32 `json:"uid"`
	GID         uint32 `json:"gid"`
	Description string `json:"description"`
}

type groupIdentity struct {
	Name string `json:"name"`
	GID  uint32 `json:"gid"`
}

type identityList struct {
	User  []userIdentity  `json:"user"`
	Group []groupIdentity `json:"group"`
}

type UsersCommand struct{ Name string }

const usersCommandName = "users"

func NewUsersCommand() *UsersCommand {
	return &UsersCommand{Name: usersCommandName}
}

func (self *UsersCommand) Run(root *os.Root, _ options) (any, error) {
	return self.read(root), nil
}

func (self *UsersCommand) read(root *os.Root) identityList {
	result := identityList{User: make([]userIdentity, 0), Group: make([]groupIdentity, 0)}
	if content, err := root.ReadFile("etc/passwd"); err == nil {
		for _, line := range strings.Split(string(content), "\n") {
			fields := strings.Split(line, ":")
			if len(fields) < 7 || fields[0] == "" {
				continue
			}
			uid, uidErr := strconv.ParseUint(fields[2], 10, 32)
			gid, gidErr := strconv.ParseUint(fields[3], 10, 32)
			if uidErr == nil && gidErr == nil {
				result.User = append(result.User, userIdentity{Name: fields[0], UID: uint32(uid), GID: uint32(gid), Description: fields[4]})
			}
		}
	}
	if content, err := root.ReadFile("etc/group"); err == nil {
		for _, line := range strings.Split(string(content), "\n") {
			fields := strings.Split(line, ":")
			if len(fields) < 3 || fields[0] == "" {
				continue
			}
			if gid, err := strconv.ParseUint(fields[2], 10, 32); err == nil {
				result.Group = append(result.Group, groupIdentity{Name: fields[0], GID: uint32(gid)})
			}
		}
	}
	return result
}
