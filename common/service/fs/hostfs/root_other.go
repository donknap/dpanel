//go:build !windows

package hostfs

import "os"

func openSystemRoots() (map[string]*os.Root, error) {
	root, err := os.OpenRoot("/")
	if err != nil {
		return nil, err
	}
	return map[string]*os.Root{"/": root}, nil
}
