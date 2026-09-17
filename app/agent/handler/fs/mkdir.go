package fs

import (
	"errors"
	"fmt"
	"os"
)

type MkdirCommand struct{ Name string }

const mkdirCommandName = "mkdir"

func NewMkdirCommand() *MkdirCommand {
	return &MkdirCommand{Name: mkdirCommandName}
}

func (self *MkdirCommand) Run(root *os.Root, option options) (any, error) {
	name, err := normalizePath(option.path)
	if err != nil {
		return nil, err
	}
	mode, err := parseMode(option.mode)
	if err != nil {
		return nil, err
	}
	_, err = root.Lstat(name)
	created := errors.Is(err, os.ErrNotExist)
	if err != nil && !created {
		return nil, fmt.Errorf("lstat directory %q: %w", option.path, err)
	}
	if err = root.MkdirAll(name, mode.Perm()); err != nil {
		return nil, fmt.Errorf("mkdir %q: %w", option.path, err)
	}
	if created {
		if err = root.Chmod(name, mode); err != nil {
			return nil, fmt.Errorf("chmod created directory %q: %w", option.path, err)
		}
	}
	return nil, nil
}
