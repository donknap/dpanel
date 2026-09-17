package fs

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
)

type ChownCommand struct{ Name string }

const chownCommandName = "chown"

func NewChownCommand() *ChownCommand {
	return &ChownCommand{Name: chownCommandName}
}

func (self *ChownCommand) Run(root *os.Root, option options) (any, error) {
	if option.recursive && option.path == "/" {
		return nil, errors.New("refusing to recursively chown root directory")
	}
	name, err := normalizePath(option.path)
	if err != nil {
		return nil, err
	}
	uid, gid := -1, -1
	if option.uid != "" {
		value, parseErr := strconv.ParseUint(option.uid, 10, 32)
		if parseErr != nil || strconv.FormatUint(value, 10) != option.uid {
			return nil, fmt.Errorf("invalid uid %q", option.uid)
		}
		uid = int(value)
	}
	if option.gid != "" {
		value, parseErr := strconv.ParseUint(option.gid, 10, 32)
		if parseErr != nil || strconv.FormatUint(value, 10) != option.gid {
			return nil, fmt.Errorf("invalid gid %q", option.gid)
		}
		gid = int(value)
	}
	if uid == -1 && gid == -1 {
		return nil, errors.New("chown requires --uid or --gid")
	}
	var chown func(string) error
	chown = func(filePath string) error {
		info, err := root.Lstat(filePath)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if option.recursive && info.IsDir() {
			entries, err := readDirectory(root, filePath)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if err = chown(path.Join(filePath, entry.Name())); err != nil {
					return err
				}
			}
		}
		return root.Chown(filePath, uid, gid)
	}
	err = chown(name)
	if err != nil {
		return nil, fmt.Errorf("chown %q: %w", option.path, err)
	}
	return nil, nil
}
