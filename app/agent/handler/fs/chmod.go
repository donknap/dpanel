package fs

import (
	"errors"
	"fmt"
	"os"
	"path"
)

type ChmodCommand struct{ Name string }

const chmodCommandName = "chmod"

func NewChmodCommand() *ChmodCommand {
	return &ChmodCommand{Name: chmodCommandName}
}

func (self *ChmodCommand) Run(root *os.Root, option options) (any, error) {
	if option.recursive && option.path == "/" {
		return nil, errors.New("refusing to recursively chmod root directory")
	}
	name, err := normalizePath(option.path)
	if err != nil {
		return nil, err
	}
	mode, err := parseMode(option.mode)
	if err != nil {
		return nil, err
	}
	if !option.recursive {
		if err = root.Chmod(name, mode); err != nil {
			return nil, fmt.Errorf("chmod %q: %w", option.path, err)
		}
		return nil, nil
	}
	var chmod func(string) error
	chmod = func(filePath string) error {
		info, err := root.Lstat(filePath)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if info.IsDir() {
			entries, err := readDirectory(root, filePath)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if err = chmod(path.Join(filePath, entry.Name())); err != nil {
					return err
				}
			}
		}
		return root.Chmod(filePath, mode)
	}
	err = chmod(name)
	if err != nil {
		return nil, fmt.Errorf("chmod %q: %w", option.path, err)
	}
	return nil, nil
}
