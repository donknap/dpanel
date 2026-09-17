package fs

import (
	"fmt"
	"os"
)

type MvCommand struct {
	Name string
	cp   *CpCommand
}

const mvCommandName = "mv"

func NewMvCommand(cp *CpCommand) *MvCommand {
	return &MvCommand{Name: mvCommandName, cp: cp}
}

func (self *MvCommand) Run(root *os.Root, option options) (any, error) {
	source, err := normalizePath(option.source)
	if err != nil {
		return nil, fmt.Errorf("invalid source path: %w", err)
	}
	target, err := normalizePath(option.target)
	if err != nil {
		return nil, fmt.Errorf("invalid target path: %w", err)
	}
	if err = self.cp.copyPath(root, source, target, true, option.overwrite); err != nil {
		return nil, fmt.Errorf("mv %q to %q: %w", option.source, option.target, err)
	}
	return nil, nil
}
