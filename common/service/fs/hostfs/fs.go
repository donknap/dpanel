package hostfs

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	serviceafs "github.com/donknap/dpanel/common/service/fs/afs"
	"github.com/donknap/dpanel/common/types/fs"
	"github.com/pkg/sftp"
	"github.com/spf13/afero"
)

var _ serviceafs.Fs = (*Fs)(nil)

type Fs struct {
	name       string
	roots      map[string]*os.Root
	workingDir string
	remoteFs   afero.Fs
	sftpClient *sftp.Client
}

func New(options ...Option) (*Fs, error) {
	fileSystem := &Fs{name: "host", workingDir: "/"}
	for _, option := range options {
		if err := option(fileSystem); err != nil {
			_ = fileSystem.Close()
			return nil, err
		}
	}
	if len(fileSystem.roots) == 0 && fileSystem.remoteFs == nil {
		if err := WithRoot("/")(fileSystem); err != nil {
			return nil, err
		}
	}
	if _, err := fileSystem.pathName(fileSystem.workingDir); err != nil {
		_ = fileSystem.Close()
		return nil, fmt.Errorf("invalid working directory: %w", err)
	}
	return fileSystem, nil
}

func (self *Fs) Name() string {
	return self.name
}

func (self *Fs) Close() error {
	var err error
	for _, root := range self.roots {
		err = errors.Join(err, root.Close())
	}
	return err
}

func (self *Fs) Destroy() error {
	return self.Close()
}

func (self *Fs) Create(name string) (afero.File, error) {
	if len(self.roots) == 0 {
		name, err := self.pathName(name)
		if err != nil {
			return nil, pathError("create", name, err)
		}
		return self.remoteFs.Create(name)
	}
	root, name, err := self.resolveLocalPath(name)
	if err != nil {
		return nil, pathError("create", name, err)
	}
	return root.Create(name)
}

func (self *Fs) Mkdir(name string, perm os.FileMode) error {
	if len(self.roots) == 0 {
		name, err := self.pathName(name)
		if err != nil {
			return pathError("mkdir", name, err)
		}
		return self.remoteFs.Mkdir(name, perm)
	}
	root, name, err := self.resolveLocalPath(name)
	if err != nil {
		return pathError("mkdir", name, err)
	}
	return root.Mkdir(name, perm)
}

func (self *Fs) MkdirAll(name string, perm os.FileMode) error {
	if len(self.roots) == 0 {
		name, err := self.pathName(name)
		if err != nil {
			return pathError("mkdir", name, err)
		}
		return self.remoteFs.MkdirAll(name, perm)
	}
	root, name, err := self.resolveLocalPath(name)
	if err != nil {
		return pathError("mkdir", name, err)
	}
	return root.MkdirAll(name, perm)
}

func (self *Fs) Open(name string) (afero.File, error) {
	if len(self.roots) == 0 {
		name, err := self.pathName(name)
		if err != nil {
			return nil, pathError("open", name, err)
		}
		return self.remoteFs.Open(name)
	}
	root, name, err := self.resolveLocalPath(name)
	if err != nil {
		return nil, pathError("open", name, err)
	}
	return root.Open(name)
}

func (self *Fs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	if len(self.roots) == 0 {
		name, err := self.pathName(name)
		if err != nil {
			return nil, pathError("openfile", name, err)
		}
		return self.remoteFs.OpenFile(name, flag, perm)
	}
	root, name, err := self.resolveLocalPath(name)
	if err != nil {
		return nil, pathError("openfile", name, err)
	}
	return root.OpenFile(name, flag, perm.Perm())
}

func (self *Fs) ReadDir(name string) ([]os.FileInfo, error) {
	if self.sftpClient != nil {
		name, err := self.pathName(name)
		if err != nil {
			return nil, pathError("readdir", name, err)
		}
		return self.sftpClient.ReadDir(name)
	}
	virtualName, err := self.pathName(name)
	if err != nil {
		return nil, pathError("readdir", name, err)
	}
	if self.hasRootDirs() && virtualName == "/" {
		rootDirs, err := self.RootDirs()
		if err != nil {
			return nil, err
		}
		result := make([]os.FileInfo, 0, len(rootDirs))
		for _, rootDir := range rootDirs {
			info, err := self.roots[rootDir].Stat(".")
			if err != nil {
				return nil, err
			}
			result = append(result, namedFileInfo{FileInfo: info, name: strings.TrimPrefix(rootDir, "/")})
		}
		return result, nil
	}
	return afero.ReadDir(self, virtualName)
}

func (self *Fs) List(name string) ([]*fs.FileData, error) {
	name, err := self.pathName(name)
	if err != nil {
		return nil, pathError("list", name, err)
	}
	entries, err := self.ReadDir(name)
	if err != nil {
		return nil, err
	}
	users, groups := self.identityNames()
	result := make([]*fs.FileData, 0, len(entries))
	for _, entry := range entries {
		result = append(result, self.fileData(path.Join(name, entry.Name()), entry, users, groups))
	}
	return result, nil
}

func (self *Fs) Info(name string) (*fs.FileData, error) {
	info, err := self.Stat(name)
	if err != nil {
		return nil, err
	}
	users, groups := self.identityNames()
	return self.fileData(name, info, users, groups), nil
}

func (self *Fs) WorkingDir() string {
	return self.workingDir
}

func (self *Fs) RootDirs() ([]string, error) {
	if !self.hasRootDirs() {
		return nil, nil
	}
	result := make([]string, 0, len(self.roots))
	for rootDir := range self.roots {
		result = append(result, rootDir)
	}
	sort.Strings(result)
	return result, nil
}

type namedFileInfo struct {
	os.FileInfo
	name string
}

func (self namedFileInfo) Name() string {
	return self.name
}

type slashRootFileInfo struct{}

func (slashRootFileInfo) Name() string       { return "/" }
func (slashRootFileInfo) Size() int64        { return 0 }
func (slashRootFileInfo) Mode() os.FileMode  { return os.ModeDir | 0o555 }
func (slashRootFileInfo) ModTime() time.Time { return time.Time{} }
func (slashRootFileInfo) IsDir() bool        { return true }
func (slashRootFileInfo) Sys() any           { return nil }

func (self *Fs) identityNames() (map[uint32]string, map[uint32]string) {
	identities, _ := self.Users()
	users := make(map[uint32]string, len(identities.User))
	groups := make(map[uint32]string, len(identities.Group))
	for _, item := range identities.User {
		users[item.UID] = item.Name
	}
	for _, item := range identities.Group {
		groups[item.GID] = item.Name
	}
	return users, groups
}

func (self *Fs) fileData(name string, info os.FileInfo, users, groups map[uint32]string) *fs.FileData {
	data := &fs.FileData{
		Path: name, Name: info.Name(), Size: info.Size(), Mod: info.Mode(), ModStr: info.Mode().String(),
		ModTime: info.ModTime(), Change: fs.ChangeDefault, IsDir: info.IsDir(), IsSymlink: info.Mode()&os.ModeSymlink != 0,
	}
	switch stat := info.Sys().(type) {
	case *sftp.FileStat:
		data.UID, data.GID = stat.UID, stat.GID
	default:
		data.UID, data.GID = serviceafs.FileOwner(info)
	}
	data.User, data.Group = users[data.UID], groups[data.GID]
	if data.User == "" {
		data.User = fmt.Sprint(data.UID)
	}
	if data.Group == "" {
		data.Group = fmt.Sprint(data.GID)
	}
	if data.IsSymlink {
		data.LinkName, _ = self.ReadlinkIfPossible(name)
	}
	return data
}

func (self *Fs) Remove(name string) error {
	if len(self.roots) == 0 {
		name, err := self.pathName(name)
		if err != nil {
			return pathError("remove", name, err)
		}
		return self.remoteFs.Remove(name)
	}
	if self.isRootDir(name) {
		return pathError("remove", name, errors.New("refusing to remove root directory"))
	}
	root, name, err := self.resolveLocalPath(name)
	if err != nil {
		return pathError("remove", name, err)
	}
	return root.Remove(name)
}

func (self *Fs) Stat(name string) (os.FileInfo, error) {
	if len(self.roots) == 0 {
		name, err := self.pathName(name)
		if err != nil {
			return nil, pathError("stat", name, err)
		}
		return self.remoteFs.Stat(name)
	}
	virtualName, err := self.pathName(name)
	if err != nil {
		return nil, pathError("stat", name, err)
	}
	if self.hasRootDirs() && virtualName == "/" {
		return slashRootFileInfo{}, nil
	}
	root, name, err := self.resolveLocalPath(virtualName)
	if err != nil {
		return nil, pathError("stat", name, err)
	}
	return root.Stat(name)
}

func (self *Fs) Chmod(name string, mode os.FileMode) error {
	if len(self.roots) == 0 {
		name, err := self.pathName(name)
		if err != nil {
			return pathError("chmod", name, err)
		}
		return self.remoteFs.Chmod(name, mode)
	}
	root, name, err := self.resolveLocalPath(name)
	if err != nil {
		return pathError("chmod", name, err)
	}
	return root.Chmod(name, mode)
}

func (self *Fs) ChmodAll(name string, mode os.FileMode, recursive bool) error {
	name, err := self.pathName(name)
	if err != nil {
		return pathError("chmod", name, err)
	}
	if recursive && self.isRootDir(name) {
		return pathError("chmod", name, errors.New("refusing to recursively chmod root directory"))
	}
	return self.changeAll(name, recursive, func(filePath string) error {
		return self.Chmod(filePath, mode)
	})
}

func (self *Fs) Chown(name string, uid, gid int) error {
	if len(self.roots) == 0 {
		name, err := self.pathName(name)
		if err != nil {
			return pathError("chown", name, err)
		}
		return self.remoteFs.Chown(name, uid, gid)
	}
	root, name, err := self.resolveLocalPath(name)
	if err != nil {
		return pathError("chown", name, err)
	}
	return root.Chown(name, uid, gid)
}

func (self *Fs) ChownAll(name string, uid, gid *int, recursive bool) error {
	if uid == nil && gid == nil {
		return errors.New("chown requires uid or gid")
	}
	name, err := self.pathName(name)
	if err != nil {
		return pathError("chown", name, err)
	}
	if recursive && self.isRootDir(name) {
		return pathError("chown", name, errors.New("refusing to recursively chown root directory"))
	}
	ownerUID, ownerGID := -1, -1
	if uid != nil {
		ownerUID = *uid
	}
	if gid != nil {
		ownerGID = *gid
	}
	return self.changeAll(name, recursive, func(filePath string) error {
		return self.Chown(filePath, ownerUID, ownerGID)
	})
}

func (self *Fs) changeAll(name string, recursive bool, change func(string) error) error {
	info, err := self.lstat(name)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	if recursive && info.IsDir() {
		entries, err := self.ReadDir(name)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err = self.changeAll(path.Join(name, entry.Name()), true, change); err != nil {
				return err
			}
		}
	}
	return change(name)
}

func (self *Fs) Chtimes(name string, atime, mtime time.Time) error {
	if len(self.roots) == 0 {
		name, err := self.pathName(name)
		if err != nil {
			return pathError("chtimes", name, err)
		}
		return self.remoteFs.Chtimes(name, atime, mtime)
	}
	root, name, err := self.resolveLocalPath(name)
	if err != nil {
		return pathError("chtimes", name, err)
	}
	return root.Chtimes(name, atime, mtime)
}

func (self *Fs) PathSize(name string) (int64, error) {
	info, err := self.lstat(name)
	if err != nil {
		return 0, err
	}
	if !info.IsDir() {
		return info.Size(), nil
	}

	seen := make(map[serviceafs.FileID]struct{})
	var size int64
	var walk func(string) error
	walk = func(filePath string) error {
		info, err := self.lstat(filePath)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if info.Mode().IsRegular() {
			if stat, err := self.fileStat(filePath, info); err != nil {
				return err
			} else if stat != nil {
				id := stat.ID
				if _, exists := seen[id]; exists {
					return nil
				}
				seen[id] = struct{}{}
			}
			size += info.Size()
			return nil
		}
		if !info.IsDir() {
			return nil
		}
		entries, err := self.ReadDir(filePath)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err = walk(path.Join(filePath, entry.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	err = walk(name)
	return size, err
}

func (self *Fs) Users() (fs.IdentityList, error) {
	result := fs.IdentityList{User: make([]fs.UserIdentity, 0), Group: make([]fs.GroupIdentity, 0)}
	if content, err := afero.ReadFile(self, "/etc/passwd"); err == nil {
		for _, line := range strings.Split(string(content), "\n") {
			fields := strings.Split(line, ":")
			if len(fields) < 7 || fields[0] == "" {
				continue
			}
			uid, uidErr := strconv.ParseUint(fields[2], 10, 32)
			gid, gidErr := strconv.ParseUint(fields[3], 10, 32)
			if uidErr == nil && gidErr == nil {
				result.User = append(result.User, fs.UserIdentity{Name: fields[0], UID: uint32(uid), GID: uint32(gid), Description: fields[4]})
			}
		}
	}
	if content, err := afero.ReadFile(self, "/etc/group"); err == nil {
		for _, line := range strings.Split(string(content), "\n") {
			fields := strings.Split(line, ":")
			if len(fields) < 3 || fields[0] == "" {
				continue
			}
			if gid, err := strconv.ParseUint(fields[2], 10, 32); err == nil {
				result.Group = append(result.Group, fs.GroupIdentity{Name: fields[0], GID: uint32(gid)})
			}
		}
	}
	return result, nil
}

func (self *Fs) Copy(source, target string, overwrite bool) error {
	source, target, _, err := self.prepareTransfer(source, target, overwrite)
	if err != nil {
		return err
	}
	return self.copyEntry(source, target, overwrite)
}

func (self *Fs) Move(source, target string, overwrite bool) error {
	source, target, targetExists, err := self.prepareTransfer(source, target, overwrite)
	if err != nil {
		return err
	}
	if !targetExists {
		if err = self.Rename(source, target); err == nil {
			return nil
		} else if !errors.Is(err, syscall.EXDEV) {
			return err
		}
	}
	if err = self.copyEntry(source, target, overwrite); err != nil {
		return err
	}
	return self.removeEntry(source)
}

func (self *Fs) prepareTransfer(source, target string, overwrite bool) (string, string, bool, error) {
	source, err := self.pathName(source)
	if err != nil {
		return "", "", false, pathError("copy", source, err)
	}
	target, err = self.pathName(target)
	if err != nil {
		return "", "", false, pathError("copy", target, err)
	}
	if source == target {
		return "", "", false, errors.New("source and target are the same")
	}
	if self.isRootDir(source) || self.isRootDir(target) {
		return "", "", false, errors.New("filesystem root cannot be copied or moved")
	}
	sourceInfo, err := self.lstat(source)
	if err != nil {
		return "", "", false, err
	}
	directories, err := self.preflightSource(source)
	if err != nil {
		return "", "", false, fmt.Errorf("inspect source: %w", err)
	}
	targetInfo, targetExists, err := self.pathInfo(target)
	if err != nil {
		return "", "", false, err
	}
	if targetExists && !overwrite {
		return "", "", false, os.ErrExist
	}
	parent, err := self.Stat(path.Dir(target))
	if err != nil {
		return "", "", false, err
	}
	if !parent.IsDir() {
		return "", "", false, errors.New("target parent is not a directory")
	}
	if err = self.preflightCopyTargets(source, target); err != nil {
		return "", "", false, fmt.Errorf("inspect copy target: %w", err)
	}
	if err = self.checkCopyAliases(source, target, sourceInfo, targetInfo, targetExists, parent, directories); err != nil {
		return "", "", false, err
	}
	if err = self.preflightTargetAliases(source, target, targetExists); err != nil {
		return "", "", false, err
	}
	return source, target, targetExists, nil
}

func (self *Fs) RemoveAll(name string) error {
	virtualName, err := self.pathName(name)
	if err != nil {
		return pathError("remove_all", name, err)
	}
	if self.isRootDir(virtualName) {
		return pathError("remove_all", virtualName, errors.New("refusing to remove root directory"))
	}
	if len(self.roots) != 0 {
		root, localName, err := self.resolveLocalPath(virtualName)
		if err != nil {
			return pathError("remove_all", name, err)
		}
		return root.RemoveAll(localName)
	}
	return self.removeEntry(virtualName)
}

func (self *Fs) Rename(oldname, newname string) error {
	if len(self.roots) != 0 {
		if self.isRootDir(oldname) || self.isRootDir(newname) {
			return &os.LinkError{Op: "rename", Old: oldname, New: newname, Err: errors.New("filesystem root cannot be renamed")}
		}
		oldRoot, localOldName, err := self.resolveLocalPath(oldname)
		if err != nil {
			return pathError("rename", oldname, err)
		}
		newRoot, localNewName, err := self.resolveLocalPath(newname)
		if err != nil {
			return pathError("rename", newname, err)
		}
		if oldRoot != newRoot {
			return &os.LinkError{Op: "rename", Old: oldname, New: newname, Err: syscall.EXDEV}
		}
		return oldRoot.Rename(localOldName, localNewName)
	}
	oldname, err := self.pathName(oldname)
	if err != nil {
		return pathError("rename", oldname, err)
	}
	newname, err = self.pathName(newname)
	if err != nil {
		return pathError("rename", newname, err)
	}
	err = self.remoteFs.Rename(oldname, newname)
	if err != nil && self.sftpClient != nil && !errors.Is(err, syscall.EXDEV) {
		return errors.Join(syscall.EXDEV, err)
	}
	return err
}

func (self *Fs) copyEntry(source, target string, overwrite bool) error {
	info, err := self.lstat(source)
	if err != nil {
		return err
	}
	targetInfo, targetExists, err := self.pathInfo(target)
	if err != nil {
		return err
	}
	if targetExists && !overwrite {
		return os.ErrExist
	}
	mergeDirectory := targetExists && info.IsDir() && targetInfo.IsDir()
	if mergeDirectory {
		entries, err := self.ReadDir(source)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err = self.copyEntry(path.Join(source, entry.Name()), path.Join(target, entry.Name()), overwrite); err != nil {
				return err
			}
		}
		if err = self.Chmod(target, info.Mode()); err != nil {
			return err
		}
		return self.Chtimes(target, info.ModTime(), info.ModTime())
	}
	return self.replaceEntry(source, target, targetExists)
}

func (self *Fs) copyEntryRaw(source, target string) error {
	info, err := self.lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		linkName, err := self.ReadlinkIfPossible(source)
		if err != nil {
			return err
		}
		return self.SymlinkIfPossible(linkName, target)
	}
	if info.IsDir() {
		if err = self.Mkdir(target, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := self.ReadDir(source)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err = self.copyEntryRaw(path.Join(source, entry.Name()), path.Join(target, entry.Name())); err != nil {
				return err
			}
		}
		if err = self.Chmod(target, info.Mode()); err != nil {
			return err
		}
		return self.Chtimes(target, info.ModTime(), info.ModTime())
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("unsupported file type %s", info.Mode().Type())
	}
	sourceFile, err := self.Open(source)
	if err != nil {
		return err
	}
	targetFile, err := self.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode())
	if err != nil {
		_ = sourceFile.Close()
		return err
	}
	_, err = io.Copy(targetFile, sourceFile)
	targetCloseErr := targetFile.Close()
	sourceCloseErr := sourceFile.Close()
	if err != nil || targetCloseErr != nil || sourceCloseErr != nil {
		return errors.Join(err, targetCloseErr, sourceCloseErr)
	}
	if err = self.Chmod(target, info.Mode()); err != nil {
		return err
	}
	return self.Chtimes(target, info.ModTime(), info.ModTime())
}

func (self *Fs) replaceEntry(source, target string, targetExists bool) (err error) {
	staging, err := self.unusedSibling(target, "copy")
	if err != nil {
		return err
	}
	if err = self.copyEntryRaw(source, staging); err != nil {
		return errors.Join(err, self.removeEntry(staging))
	}
	if !targetExists {
		if err = self.Rename(staging, target); err != nil {
			return errors.Join(err, self.removeEntry(staging))
		}
		return nil
	}
	backup, err := self.unusedSibling(target, "backup")
	if err != nil {
		return errors.Join(err, self.removeEntry(staging))
	}
	if err = self.Rename(target, backup); err != nil {
		return errors.Join(err, self.removeEntry(staging))
	}
	if err = self.Rename(staging, target); err != nil {
		rollbackErr := self.Rename(backup, target)
		cleanupErr := self.removeEntry(staging)
		if rollbackErr != nil {
			return fmt.Errorf("commit copy: %w; rollback failed and the original target remains at %q: %v", err, backup, rollbackErr)
		}
		return errors.Join(err, cleanupErr)
	}
	if err = self.removeEntry(backup); err != nil {
		return fmt.Errorf("remove copy backup %q: %w", backup, err)
	}
	return nil
}

func (self *Fs) unusedSibling(target, purpose string) (string, error) {
	for range 16 {
		value := make([]byte, 8)
		if _, err := rand.Read(value); err != nil {
			return "", err
		}
		name := path.Join(path.Dir(target), ".dpanel-"+purpose+"-"+hex.EncodeToString(value))
		_, exists, err := self.pathInfo(name)
		if err != nil {
			return "", err
		}
		if !exists {
			return name, nil
		}
	}
	return "", errors.New("cannot allocate a temporary copy path")
}

func (self *Fs) preflightSource(source string) (map[serviceafs.FileID]struct{}, error) {
	directories := make(map[serviceafs.FileID]struct{})
	var walk func(string) error
	walk = func(name string) error {
		info, err := self.lstat(name)
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			linkName, err := self.ReadlinkIfPossible(name)
			if err != nil {
				return err
			}
			if len(self.roots) != 0 && filepath.IsAbs(linkName) {
				return errors.New("absolute symbolic link cannot be copied")
			}
			_, err = self.symlinkTarget(linkName, name)
			return err
		case info.Mode().IsRegular():
			return nil
		case info.IsDir():
			if stat, err := self.fileStat(name, info); err != nil {
				return err
			} else if stat != nil {
				if _, exists := directories[stat.ID]; exists {
					return errors.New("source directory contains a filesystem cycle")
				}
				directories[stat.ID] = struct{}{}
			}
			entries, err := self.ReadDir(name)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if err = walk(path.Join(name, entry.Name())); err != nil {
					return err
				}
			}
			return nil
		default:
			return fmt.Errorf("unsupported file type %s", info.Mode().Type())
		}
	}
	return directories, walk(source)
}

func (self *Fs) preflightCopyTargets(source, target string) error {
	info, err := self.lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		linkName, err := self.ReadlinkIfPossible(source)
		if err != nil {
			return err
		}
		if len(self.roots) != 0 && filepath.IsAbs(linkName) {
			return errors.New("absolute symbolic link cannot be copied")
		}
		_, err = self.symlinkTarget(linkName, target)
		return err
	}
	if !info.IsDir() {
		return nil
	}
	entries, err := self.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err = self.preflightCopyTargets(path.Join(source, entry.Name()), path.Join(target, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func (self *Fs) checkCopyAliases(source, target string, sourceInfo, targetInfo os.FileInfo, targetExists bool, parent os.FileInfo, directories map[serviceafs.FileID]struct{}) error {
	if self.sftpClient != nil {
		realSource, err := self.sftpClient.RealPath(source)
		if err != nil && sourceInfo.IsDir() {
			return err
		}
		realParent, err := self.sftpClient.RealPath(path.Dir(target))
		if err != nil {
			return err
		}
		realTarget := path.Join(realParent, path.Base(target))
		if targetExists {
			if resolved, resolveErr := self.sftpClient.RealPath(target); resolveErr == nil {
				realTarget = resolved
			}
		}
		if realSource != "" && (realTarget == realSource || sourceInfo.IsDir() && strings.HasPrefix(realTarget, strings.TrimSuffix(realSource, "/")+"/")) {
			return errors.New("a directory cannot be copied or moved into itself")
		}
		return nil
	}
	if targetExists {
		sourceStat, err := self.fileStat(source, sourceInfo)
		if err != nil {
			return err
		}
		targetStat, err := self.fileStat(target, targetInfo)
		if err != nil {
			return err
		}
		if sourceStat != nil && targetStat != nil && sourceStat.ID == targetStat.ID {
			return errors.New("source and target are the same file")
		}
	}
	if sourceInfo.IsDir() {
		if stat, err := self.fileStat(path.Dir(target), parent); err != nil {
			return err
		} else if stat != nil {
			if _, exists := directories[stat.ID]; exists {
				return errors.New("a directory cannot be copied or moved into itself")
			}
		}
		if targetExists {
			resolvedTarget, err := self.Stat(target)
			if err == nil {
				if stat, err := self.fileStat(target, resolvedTarget); err != nil {
					return err
				} else if stat != nil {
					if _, exists := directories[stat.ID]; exists {
						return errors.New("a directory cannot be copied or moved into itself")
					}
				}
			}
		}
	}
	return nil
}

func (self *Fs) preflightTargetAliases(source, target string, targetExists bool) error {
	if !targetExists || len(self.roots) == 0 {
		return nil
	}
	sourceInfo, err := self.lstat(source)
	if err != nil {
		return err
	}
	targetInfo, err := self.lstat(target)
	if err != nil {
		return err
	}
	sourceStat, err := self.fileStat(source, sourceInfo)
	if err != nil {
		return err
	}
	targetStat, err := self.fileStat(target, targetInfo)
	if err != nil {
		return err
	}
	if sourceStat != nil && targetStat != nil && sourceStat.ID == targetStat.ID {
		return errors.New("source and target are the same file")
	}
	if !sourceInfo.IsDir() || !targetInfo.IsDir() {
		return nil
	}
	entries, err := self.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sourceChild := path.Join(source, entry.Name())
		targetChild := path.Join(target, entry.Name())
		_, exists, err := self.pathInfo(targetChild)
		if err != nil {
			return err
		}
		if exists {
			if err = self.preflightTargetAliases(sourceChild, targetChild, true); err != nil {
				return err
			}
		}
	}
	return nil
}

func (self *Fs) fileStat(name string, info os.FileInfo) (*serviceafs.FileStat, error) {
	// SFTP metadata is supplied by the remote server, not the panel's OS.
	if len(self.roots) == 0 {
		return nil, nil
	}
	root, name, err := self.resolveLocalPath(name)
	if err != nil {
		return nil, err
	}
	return serviceafs.ReadFileStat(root, name, info)
}

func (self *Fs) removeEntry(name string) error {
	if self.isRootDir(name) {
		return pathError("remove", name, errors.New("refusing to remove root directory"))
	}
	info, exists, err := self.pathInfo(name)
	if err != nil || !exists {
		return err
	}
	if info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
		entries, err := self.ReadDir(name)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err = self.removeEntry(path.Join(name, entry.Name())); err != nil {
				return err
			}
		}
	}
	return self.Remove(name)
}

func (self *Fs) pathInfo(name string) (os.FileInfo, bool, error) {
	info, err := self.lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	return info, err == nil, err
}

func (self *Fs) lstat(name string) (os.FileInfo, error) {
	info, _, err := self.LstatIfPossible(name)
	return info, err
}

func (self *Fs) LstatIfPossible(name string) (os.FileInfo, bool, error) {
	if len(self.roots) != 0 {
		virtualName, err := self.pathName(name)
		if err != nil {
			return nil, true, pathError("lstat", name, err)
		}
		if self.hasRootDirs() && virtualName == "/" {
			return slashRootFileInfo{}, true, nil
		}
		root, localName, err := self.resolveLocalPath(virtualName)
		if err != nil {
			return nil, true, pathError("lstat", name, err)
		}
		info, err := root.Lstat(localName)
		return info, true, err
	}
	if self.sftpClient != nil {
		name, err := self.pathName(name)
		if err != nil {
			return nil, true, err
		}
		info, err := self.sftpClient.Lstat(name)
		return info, true, err
	}
	return nil, false, errors.New("filesystem backend is unavailable")
}

func (self *Fs) SymlinkIfPossible(oldname, newname string) error {
	virtualNewName, err := self.pathName(newname)
	if err != nil {
		return pathError("symlink", newname, err)
	}
	oldname, err = self.symlinkTarget(oldname, virtualNewName)
	if err != nil {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: err}
	}
	if len(self.roots) != 0 {
		root, localNewName, err := self.resolveLocalPath(virtualNewName)
		if err != nil {
			return pathError("symlink", newname, err)
		}
		return root.Symlink(filepath.FromSlash(oldname), localNewName)
	}
	if self.sftpClient != nil {
		return self.sftpClient.Symlink(oldname, virtualNewName)
	}
	return afero.ErrNoSymlink
}

func (self *Fs) ReadlinkIfPossible(name string) (string, error) {
	if len(self.roots) != 0 {
		root, name, err := self.resolveLocalPath(name)
		if err != nil {
			return "", pathError("readlink", name, err)
		}
		linkName, err := root.Readlink(name)
		return filepath.ToSlash(linkName), err
	}
	if self.sftpClient != nil {
		name, err := self.pathName(name)
		if err != nil {
			return "", err
		}
		return self.sftpClient.ReadLink(name)
	}
	return "", afero.ErrNoReadlink
}

func (self *Fs) pathName(name string) (string, error) {
	if name == "" {
		name = self.workingDir
	}
	if strings.IndexByte(name, 0) >= 0 || !path.IsAbs(name) || path.Clean(name) != name {
		return name, errors.New("invalid path")
	}
	return name, nil
}

func (self *Fs) hasRootDirs() bool {
	if len(self.roots) == 0 {
		return false
	}
	_, singleRoot := self.roots["/"]
	return !singleRoot
}

func (self *Fs) isRootDir(name string) bool {
	name, err := self.pathName(name)
	if err != nil {
		return false
	}
	if name == "/" {
		return true
	}
	if !self.hasRootDirs() {
		return false
	}
	_, exists := self.roots[strings.ToLower(name)]
	return exists
}

func (self *Fs) resolveLocalPath(name string) (*os.Root, string, error) {
	virtualName, err := self.pathName(name)
	if err != nil {
		return nil, name, err
	}
	if root := self.roots["/"]; root != nil {
		localName := strings.TrimPrefix(virtualName, "/")
		if localName == "" {
			localName = "."
		}
		return root, filepath.FromSlash(localName), nil
	}
	components := strings.SplitN(strings.TrimPrefix(virtualName, "/"), "/", 2)
	rootName := "/" + strings.ToLower(components[0])
	root := self.roots[rootName]
	if root == nil {
		return nil, virtualName, fmt.Errorf("%w: filesystem root %s", os.ErrNotExist, rootName)
	}
	localName := "."
	if len(components) == 2 {
		localName = filepath.FromSlash(components[1])
	}
	return root, localName, nil
}

func (self *Fs) symlinkTarget(oldname, newname string) (string, error) {
	if len(self.roots) != 0 {
		if filepath.VolumeName(oldname) != "" {
			return oldname, errors.New("system absolute symlink target is not allowed")
		}
		oldname = filepath.ToSlash(oldname)
	}
	if oldname == "" || strings.IndexByte(oldname, 0) >= 0 || path.Clean(oldname) != oldname {
		return oldname, errors.New("invalid symlink target")
	}
	var target string
	if path.IsAbs(oldname) {
		var err error
		target, err = self.pathName(oldname)
		if err != nil {
			return oldname, err
		}
	} else {
		components := strings.Split(strings.TrimPrefix(path.Dir(newname), "/"), "/")
		if len(components) == 1 && components[0] == "" {
			components = nil
		}
		for _, component := range strings.Split(oldname, "/") {
			switch component {
			case ".":
			case "..":
				if len(components) == 0 {
					return oldname, errors.New("symlink target escapes filesystem root")
				}
				components = components[:len(components)-1]
			default:
				components = append(components, component)
			}
		}
		target = "/" + strings.Join(components, "/")
	}
	if self.hasRootDirs() {
		targetRoot, _, err := self.resolveLocalPath(target)
		if err != nil {
			return oldname, err
		}
		newRoot, _, err := self.resolveLocalPath(newname)
		if err != nil {
			return oldname, err
		}
		if targetRoot != newRoot {
			return oldname, errors.New("symbolic link target crosses filesystem roots")
		}
	}
	relativeTarget, err := filepath.Rel(filepath.FromSlash(path.Dir(newname)), filepath.FromSlash(target))
	if err != nil {
		return oldname, err
	}
	return filepath.ToSlash(relativeTarget), nil
}

func pathError(operation, name string, err error) error {
	return &os.PathError{Op: operation, Path: name, Err: err}
}

func (self *Fs) Import(fileList []serviceafs.TransferFile) error {
	for _, item := range fileList {
		if err := validateLocalTransferPath(item.Source, true); err != nil {
			return fmt.Errorf("invalid import source %q: %w", item.Source, err)
		}
		if _, err := self.pathName(item.Target); err != nil {
			return fmt.Errorf("invalid import target %q: %w", item.Target, err)
		}
		if err := self.validateTransferTarget(item.Target); err != nil {
			return fmt.Errorf("invalid import target %q: %w", item.Target, err)
		}
	}
	for _, item := range fileList {
		if err := self.importEntry(item.Source, item.Target); err != nil {
			return err
		}
	}
	return nil
}

func (self *Fs) Export(fileList []serviceafs.TransferFile) error {
	for _, item := range fileList {
		if _, err := self.pathName(item.Source); err != nil {
			return fmt.Errorf("invalid export source %q: %w", item.Source, err)
		}
		if err := self.validateTransferSource(item.Source); err != nil {
			return fmt.Errorf("invalid export source %q: %w", item.Source, err)
		}
		if err := validateLocalTransferPath(item.Target, false); err != nil {
			return fmt.Errorf("invalid export target %q: %w", item.Target, err)
		}
	}
	for _, item := range fileList {
		if err := self.exportEntry(item.Source, item.Target); err != nil {
			return err
		}
	}
	return nil
}

func (self *Fs) validateTransferSource(name string) error {
	info, err := self.lstat(name)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() && !info.Mode().IsRegular() {
		return errors.New("source is not a regular file or directory")
	}
	if info.Mode().IsRegular() {
		if stat, err := self.fileStat(name, info); err != nil {
			return err
		} else if stat != nil && stat.Links > 1 {
			return errors.New("source is a hard link")
		}
		return nil
	}
	entries, err := self.ReadDir(name)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err = self.validateTransferSource(path.Join(name, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func (self *Fs) validateTransferTarget(name string) error {
	name, err := self.pathName(name)
	if err != nil {
		return err
	}
	current := "/"
	components := strings.Split(strings.TrimPrefix(name, "/"), "/")
	if name == "/" {
		components = nil
	}
	for index, component := range components {
		current = path.Join(current, component)
		info, err := self.lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("target contains a symbolic link")
		}
		if index < len(components)-1 && !info.IsDir() {
			return errors.New("target parent is not a directory")
		}
		if index == len(components)-1 && !info.IsDir() && !info.Mode().IsRegular() {
			return errors.New("target is not a regular file or directory")
		}
		if index == len(components)-1 && info.Mode().IsRegular() {
			if stat, err := self.fileStat(current, info); err != nil {
				return err
			} else if stat != nil && stat.Links > 1 {
				return errors.New("target is a hard link")
			}
		}
	}
	return nil
}

func (self *Fs) importEntry(source, target string) error {
	if err := self.validateTransferTarget(target); err != nil {
		return err
	}
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.IsDir() {
		targetInfo, exists, err := self.pathInfo(target)
		if err != nil {
			return err
		}
		if exists && !targetInfo.IsDir() {
			return errors.New("import directory target is not a directory")
		}
		if !exists {
			if err = self.MkdirAll(target, info.Mode().Perm()); err != nil {
				return err
			}
		}
		entries, err := os.ReadDir(source)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err = self.importEntry(filepath.Join(source, entry.Name()), path.Join(target, entry.Name())); err != nil {
				return err
			}
		}
		if len(self.roots) == 0 {
			return nil
		}
		if err = self.Chmod(target, info.Mode()); err != nil {
			return err
		}
		return self.Chtimes(target, info.ModTime(), info.ModTime())
	}
	if err = self.MkdirAll(path.Dir(target), 0o755); err != nil {
		return err
	}
	sourceFile, err := os.Open(source)
	if err != nil {
		return err
	}
	targetFile, err := self.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		_ = sourceFile.Close()
		return err
	}
	_, copyErr := io.Copy(targetFile, sourceFile)
	err = errors.Join(copyErr, targetFile.Close(), sourceFile.Close())
	if err != nil {
		return err
	}
	if err = self.Chmod(target, info.Mode()); err != nil {
		return err
	}
	return self.Chtimes(target, info.ModTime(), info.ModTime())
}

func (self *Fs) exportEntry(source, target string) error {
	info, err := self.lstat(source)
	if err != nil {
		return err
	}
	if info.IsDir() {
		targetInfo, err := os.Lstat(target)
		if err == nil && !targetInfo.IsDir() {
			return errors.New("export directory target is not a directory")
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err = os.MkdirAll(target, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := self.ReadDir(source)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err = self.exportEntry(path.Join(source, entry.Name()), filepath.Join(target, entry.Name())); err != nil {
				return err
			}
		}
		if err = os.Chmod(target, info.Mode().Perm()); err != nil {
			return err
		}
		return os.Chtimes(target, info.ModTime(), info.ModTime())
	}
	if err = os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	sourceFile, err := self.Open(source)
	if err != nil {
		return err
	}
	temporaryFile, err := os.CreateTemp(filepath.Dir(target), ".dpanel-export-*")
	if err != nil {
		_ = sourceFile.Close()
		return err
	}
	temporaryPath := temporaryFile.Name()
	defer os.Remove(temporaryPath)
	_, copyErr := io.Copy(temporaryFile, sourceFile)
	err = errors.Join(copyErr, temporaryFile.Chmod(info.Mode().Perm()), temporaryFile.Close(), sourceFile.Close())
	if err != nil {
		return err
	}
	if err = os.Chtimes(temporaryPath, info.ModTime(), info.ModTime()); err != nil {
		return err
	}
	if err = validateLocalTransferPath(target, false); err != nil {
		return err
	}
	return os.Rename(temporaryPath, target)
}

func validateLocalTransferPath(name string, mustExist bool) error {
	if name == "" || strings.IndexByte(name, 0) >= 0 || !filepath.IsAbs(name) || filepath.Clean(name) != name {
		return errors.New("local path must be a clean absolute path")
	}
	info, err := os.Lstat(name)
	if errors.Is(err, os.ErrNotExist) && !mustExist {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() && !info.Mode().IsRegular() {
		return errors.New("local path is not a regular file or directory")
	}
	if info.Mode().IsRegular() {
		if stat, err := serviceafs.ReadFileStat(nil, name, info); err != nil {
			return err
		} else if stat != nil && stat.Links > 1 {
			return errors.New("local path is a hard link")
		}
	}
	if mustExist {
		if info.IsDir() {
			entries, err := os.ReadDir(name)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if err = validateLocalTransferPath(filepath.Join(name, entry.Name()), true); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
