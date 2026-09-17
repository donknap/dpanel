package fs

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
)

func normalizePath(value string) (string, error) {
	if value == "" || strings.IndexByte(value, 0) >= 0 || !path.IsAbs(value) || path.Clean(value) != value {
		return "", errors.New("invalid path")
	}
	value = strings.TrimPrefix(value, "/")
	if value == "" {
		return ".", nil
	}
	return value, nil
}

func parseMode(value string) (os.FileMode, error) {
	if len(value) == 0 || len(value) > 4 {
		return 0, fmt.Errorf("invalid mode %q", value)
	}
	for _, char := range value {
		if char < '0' || char > '7' {
			return 0, fmt.Errorf("invalid mode %q", value)
		}
	}
	numeric, err := strconv.ParseUint(value, 8, 12)
	if err != nil || numeric > 0o7777 {
		return 0, fmt.Errorf("invalid mode %q", value)
	}
	mode := os.FileMode(numeric & 0o777)
	if numeric&0o4000 != 0 {
		mode |= os.ModeSetuid
	}
	if numeric&0o2000 != 0 {
		mode |= os.ModeSetgid
	}
	if numeric&0o1000 != 0 {
		mode |= os.ModeSticky
	}
	return mode, nil
}

func readDirectory(root *os.Root, name string) ([]os.DirEntry, error) {
	directory, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	entries, readErr := directory.ReadDir(-1)
	closeErr := directory.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return entries, nil
}
