//go:build !windows

package afs

import (
	"os"
	"syscall"
)

func FileOwner(info os.FileInfo) (uint32, uint32) {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return stat.Uid, stat.Gid
	}
	return 0, 0
}

// ReadFileStat uses the existing stat result on Unix; nil means non-native metadata.
func ReadFileStat(_ *os.Root, _ string, info os.FileInfo) (*FileStat, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, nil
	}
	return &FileStat{
		ID:    FileID{Device: uint64(stat.Dev), Inode: uint64(stat.Ino)},
		Links: uint64(stat.Nlink),
	}, nil
}
