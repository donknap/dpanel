//go:build windows

package hostfs

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

func openSystemRoots() (map[string]*os.Root, error) {
	drives, err := windows.GetLogicalDrives()
	if err != nil {
		return nil, err
	}
	roots := make(map[string]*os.Root)
	var openErr error
	for index := range 26 {
		if drives&(1<<index) == 0 {
			continue
		}
		letter := string(rune('A' + index))
		root, err := os.OpenRoot(letter + `:\`)
		if err != nil {
			openErr = errors.Join(openErr, fmt.Errorf("open drive %s: %w", letter, err))
			continue
		}
		roots["/"+strings.ToLower(letter)] = root
	}
	if len(roots) == 0 {
		if openErr == nil {
			openErr = errors.New("no accessible filesystem roots")
		}
		return nil, openErr
	}
	return roots, nil
}
