package fs

import (
	"errors"
	"fmt"
	"os"
	"time"
)

type ChtimesCommand struct{ Name string }

const chtimesCommandName = "chtimes"

func NewChtimesCommand() *ChtimesCommand {
	return &ChtimesCommand{Name: chtimesCommandName}
}

func (self *ChtimesCommand) Run(root *os.Root, option options) (any, error) {
	name, err := normalizePath(option.path)
	if err != nil {
		return nil, err
	}
	if option.atime == "" || option.mtime == "" {
		return nil, errors.New("chtimes requires --atime and --mtime")
	}
	atime, err := parseTime(option.atime)
	if err != nil {
		return nil, fmt.Errorf("invalid atime: %w", err)
	}
	mtime, err := parseTime(option.mtime)
	if err != nil {
		return nil, fmt.Errorf("invalid mtime: %w", err)
	}
	if err = root.Chtimes(name, atime, mtime); err != nil {
		return nil, fmt.Errorf("chtimes %q: %w", option.path, err)
	}
	return nil, nil
}

func parseTime(value string) (time.Time, error) {
	result, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || result.Format(time.RFC3339Nano) != value {
		return time.Time{}, fmt.Errorf("invalid RFC3339Nano time %q", value)
	}
	return result, nil
}
