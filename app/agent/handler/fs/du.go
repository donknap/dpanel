package fs

import (
	"errors"
	"fmt"
	"os"
	"path"
	"syscall"
)

type DuCommand struct{ Name string }

type duFileID struct {
	device uint64
	inode  uint64
}

const duCommandName = "du"

func NewDuCommand() *DuCommand {
	return &DuCommand{Name: duCommandName}
}

func (self *DuCommand) Run(root *os.Root, option options) (any, error) {
	directoryPath, err := normalizePath(option.path)
	if err != nil {
		return nil, err
	}
	info, err := root.Lstat(directoryPath)
	if err != nil {
		return nil, fmt.Errorf("lstat directory: %w", err)
	}
	if !info.IsDir() {
		return nil, errors.New("target is not a directory")
	}
	seen := make(map[duFileID]struct{})
	var total int64
	var walk func(string) error
	walk = func(name string) error {
		info, err := root.Lstat(name)
		if err != nil {
			return err
		}
		if info.IsDir() {
			entries, err := readDirectory(root, name)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if err = walk(path.Join(name, entry.Name())); err != nil {
					return err
				}
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return fmt.Errorf("inode is unavailable for %q", name)
		}
		id := duFileID{device: uint64(stat.Dev), inode: uint64(stat.Ino)}
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			total += info.Size()
		}
		return nil
	}
	if err = walk(directoryPath); err != nil {
		return nil, err
	}
	return map[string]int64{"size": total}, nil
}
