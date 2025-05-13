package explorer

import (
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
	Path      string      `json:"path"` // 完整的目录地址
	Name      string      `json:"name"` // 目录名
	Mod       os.FileMode `json:"mod"`
	ModStr    string      `json:"modStr"` // 权限字符形式
	ModTime   time.Time   `json:"modTime"`
	Change    ChangeType  `json:"change"`
	Size      int64       `json:"size"`
	User      string      `json:"user"`
	Group     string      `json:"group"`
	LinkName  string      `json:"linkName"`
	IsDir     bool        `json:"isDir"`
	IsSymlink bool        `json:"isSymlink"`
}

func (self *FileData) CheckIsSymlink() bool {
	return self.Mod&os.ModeSymlink != 0
}

func (self *FileData) CheckIsBlockDevice() bool {
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
