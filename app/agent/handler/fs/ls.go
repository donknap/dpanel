package fs

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"syscall"
	"time"
	"unicode/utf8"
)

type fileData struct {
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

type LsCommand struct {
	Name  string
	users *UsersCommand
}

const lsCommandName = "ls"

func NewLsCommand(users *UsersCommand) *LsCommand {
	return &LsCommand{Name: lsCommandName, users: users}
}

func (self *LsCommand) Run(root *os.Root, option options) (any, error) {
	directoryPath, err := normalizePath(option.path)
	if err != nil {
		return nil, err
	}
	entries, err := readDirectory(root, directoryPath)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}
	identities := self.users.read(root)
	users := make(map[uint32]string, len(identities.User)+1)
	groups := make(map[uint32]string, len(identities.Group)+1)
	for _, user := range identities.User {
		if _, exists := users[user.UID]; !exists {
			users[user.UID] = user.Name
		}
	}
	for _, group := range identities.Group {
		if _, exists := groups[group.GID]; !exists {
			groups[group.GID] = group.Name
		}
	}
	if _, exists := users[0]; !exists {
		users[0] = "root"
	}
	if _, exists := groups[0]; !exists {
		groups[0] = "root"
	}

	result := make([]fileData, 0, len(entries))
	for _, entry := range entries {
		if !utf8.ValidString(entry.Name()) {
			return nil, fmt.Errorf("directory contains a non-UTF-8 filename: %q", []byte(entry.Name()))
		}
		entryPath := path.Join(directoryPath, entry.Name())
		info, err := root.Lstat(entryPath)
		if err != nil {
			return nil, fmt.Errorf("lstat %q: %w", entry.Name(), err)
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return nil, fmt.Errorf("read owner for %q: owner is unavailable", entry.Name())
		}
		uid, gid := stat.Uid, stat.Gid
		item := fileData{Name: entry.Name(), Size: info.Size(), Mode: info.Mode(), ModeText: self.modeText(info.Mode()), ModTime: info.ModTime(), UID: uid, GID: gid, User: identityName(users, uid), Group: identityName(groups, gid)}
		if info.Mode()&os.ModeSymlink != 0 {
			item.LinkName, err = root.Readlink(entryPath)
			if err != nil {
				return nil, fmt.Errorf("readlink %q: %w", entry.Name(), err)
			}
			if !utf8.ValidString(item.LinkName) {
				return nil, fmt.Errorf("symlink %q has a non-UTF-8 target", entry.Name())
			}
		}
		result = append(result, item)
	}
	return result, nil
}

func (self *LsCommand) modeText(mode os.FileMode) string {
	result := []byte("----------")
	switch {
	case mode.IsDir():
		result[0] = 'd'
	case mode&os.ModeSymlink != 0:
		result[0] = 'l'
	case mode&os.ModeNamedPipe != 0:
		result[0] = 'p'
	case mode&os.ModeSocket != 0:
		result[0] = 's'
	case mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0:
		result[0] = 'c'
	case mode&os.ModeDevice != 0:
		result[0] = 'b'
	}
	permissions := []struct {
		mode os.FileMode
		char byte
	}{
		{0o400, 'r'}, {0o200, 'w'}, {0o100, 'x'},
		{0o040, 'r'}, {0o020, 'w'}, {0o010, 'x'},
		{0o004, 'r'}, {0o002, 'w'}, {0o001, 'x'},
	}
	for index, permission := range permissions {
		if mode&permission.mode != 0 {
			result[index+1] = permission.char
		}
	}
	if mode&os.ModeSetuid != 0 {
		if result[3] == 'x' {
			result[3] = 's'
		} else {
			result[3] = 'S'
		}
	}
	if mode&os.ModeSetgid != 0 {
		if result[6] == 'x' {
			result[6] = 's'
		} else {
			result[6] = 'S'
		}
	}
	if mode&os.ModeSticky != 0 {
		if result[9] == 'x' {
			result[9] = 't'
		} else {
			result[9] = 'T'
		}
	}
	return string(result)
}

func identityName(identities map[uint32]string, id uint32) string {
	if name, ok := identities[id]; ok {
		return name
	}
	return strconv.FormatUint(uint64(id), 10)
}
