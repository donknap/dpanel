package fs

import (
	"errors"
	"fmt"
	"os"
)

type RmCommand struct{ Name string }

const rmCommandName = "rm"

func NewRmCommand() *RmCommand {
	return &RmCommand{Name: rmCommandName}
}

func (self *RmCommand) Run(root *os.Root, option options) (any, error) {
	if option.path == "/" {
		return nil, errors.New("refusing to remove root directory")
	}
	name, err := normalizePath(option.path)
	if err != nil {
		return nil, err
	}
	if err = root.RemoveAll(name); err != nil {
		return nil, fmt.Errorf("rm %q: %w", option.path, err)
	}
	return nil, nil
}
