package afs

import (
	"errors"
	"os"
	"syscall"
)

// Windows files have no Unix UID/GID. Remote ownership is handled by SFTP.
func FileOwner(_ os.FileInfo) (uint32, uint32) {
	return 0, 0
}

// ReadFileStat obtains identity and link count from a handle because FileInfo.Sys on
// Windows exposes neither. Root-relative paths must stay inside os.Root.
func ReadFileStat(root *os.Root, name string, info os.FileInfo) (*FileStat, error) {
	if _, ok := info.Sys().(*syscall.Win32FileAttributeData); !ok {
		return nil, nil
	}
	var file *os.File
	var err error
	if root != nil {
		file, err = root.Open(name)
	} else {
		file, err = os.Open(name)
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(info, openedInfo) {
		return nil, &os.PathError{Op: "stat", Path: name, Err: errors.New("file changed while reading metadata")}
	}
	var stat syscall.ByHandleFileInformation
	if err = syscall.GetFileInformationByHandle(syscall.Handle(file.Fd()), &stat); err != nil {
		return nil, &os.PathError{Op: "GetFileInformationByHandle", Path: name, Err: err}
	}
	return &FileStat{
		ID:    FileID{Device: uint64(stat.VolumeSerialNumber), Inode: uint64(stat.FileIndexHigh)<<32 | uint64(stat.FileIndexLow)},
		Links: uint64(stat.NumberOfLinks),
	}, nil
}
