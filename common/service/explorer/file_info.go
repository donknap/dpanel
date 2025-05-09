package explorer

import (
	"fmt"
	"io/fs"
	"os"
	"time"
)

type ChangeType int

const (
	ChangeDefault  = -1
	ChangeModified = 0
	ChangeAdd      = 1
	ChangeDeleted  = 2
	ChangeVolume   = 100
)

func NewFileInfo(data *FileData) os.FileInfo {
	return &FileInfo{
		stat: data,
	}
}

type FileData struct {
	Name     string      `json:"name"`
	Mod      os.FileMode `json:"mod"`
	ModTime  time.Time   `json:"modTime"`
	Change   ChangeType  `json:"change"`
	Size     int64       `json:"size"`
	Owner    string      `json:"owner"`
	Group    string      `json:"group"`
	LinkName string      `json:"linkName"`
}

func (self *FileData) IsSymlink() bool {
	return self.Mod&os.ModeSymlink != 0
}

func (self *FileData) IsBlockDevice() bool {
	return self.Mod&os.ModeDevice != 0 && self.Mod&os.ModeCharDevice == 0
}

type FileInfo struct {
	os.FileInfo
	stat *FileData
}

func (self *FileInfo) Name() string {
	return self.stat.Name
}

func (self *FileInfo) Size() int64 {
	return self.stat.Size
}

func (self *FileInfo) Mode() fs.FileMode {
	return self.stat.Mod
}

func (self *FileInfo) ModTime() time.Time {
	return self.stat.ModTime
}

func (self *FileInfo) IsDir() bool {
	return self.stat.Mod.IsDir()
}

func (self *FileInfo) Sys() any {
	return self.stat
}

func (self *FileInfo) LinkName() string {
	return self.stat.LinkName
}

func (self *FileInfo) Owner() string {
	return self.stat.Owner
}

func (self *FileInfo) Group() string {
	return self.stat.Group
}

func (self *FileInfo) ShowName() string {
	if self.stat.LinkName == "" {
		return self.stat.Name
	}
	return fmt.Sprintf("%s -> %s", self.stat.Name, self.stat.LinkName)
}
