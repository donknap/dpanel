package fs

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"syscall"
)

type CpCommand struct{ Name string }

type copyFileID struct {
	device uint64
	inode  uint64
}

const cpCommandName = "cp"

func NewCpCommand() *CpCommand {
	return &CpCommand{Name: cpCommandName}
}

func (self *CpCommand) Run(root *os.Root, option options) (any, error) {
	source, err := normalizePath(option.source)
	if err != nil {
		return nil, fmt.Errorf("invalid source path: %w", err)
	}
	target, err := normalizePath(option.target)
	if err != nil {
		return nil, fmt.Errorf("invalid target path: %w", err)
	}
	if err = self.copyPath(root, source, target, false, option.overwrite); err != nil {
		return nil, fmt.Errorf("cp %q to %q: %w", option.source, option.target, err)
	}
	return nil, nil
}

func (self *CpCommand) copyPath(root *os.Root, source, target string, move, overwrite bool) error {
	if source == "." || target == "." {
		return errors.New("root directory cannot be copied or moved")
	}
	if source == target {
		return errors.New("source and target are the same")
	}
	sourceInfo, err := root.Lstat(source)
	if err != nil {
		return fmt.Errorf("lstat source: %w", err)
	}
	targetInfo, err := root.Lstat(target)
	targetExists := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("lstat target: %w", err)
	}
	if targetExists && !overwrite {
		return os.ErrExist
	}
	parentInfo, err := root.Stat(path.Dir(target))
	if err != nil {
		return fmt.Errorf("stat target parent: %w", err)
	}
	if !parentInfo.IsDir() {
		return errors.New("target parent is not a directory")
	}
	if move && !targetExists {
		if err = root.Rename(source, target); err == nil {
			return nil
		}
		if !errors.Is(err, syscall.EXDEV) {
			return err
		}
	}
	sourceDirectories, err := preflightSource(root, source)
	if err != nil {
		return fmt.Errorf("inspect source: %w", err)
	}
	if targetExists {
		if sameFile(sourceInfo, targetInfo) {
			return errors.New("source and target are the same file")
		}
		if resolvedTarget, statErr := root.Stat(target); statErr == nil && sameFile(sourceInfo, resolvedTarget) {
			return errors.New("source and target are the same file")
		}
		if err = preflightTargetAliases(root, source, target); err != nil {
			return err
		}
	}
	if sourceInfo.IsDir() {
		targetInsideSource := targetExists && directoryIDExists(sourceDirectories, targetInfo)
		if targetExists {
			if resolvedTarget, statErr := root.Stat(target); statErr == nil {
				targetInsideSource = targetInsideSource || directoryIDExists(sourceDirectories, resolvedTarget)
			}
		}
		if directoryIDExists(sourceDirectories, parentInfo) || targetInsideSource {
			return errors.New("a directory cannot be copied or moved into itself")
		}
	}
	if err = self.copyEntry(root, source, target, overwrite); err != nil {
		return err
	}
	if move {
		if err = root.RemoveAll(source); err != nil {
			return fmt.Errorf("remove source after copy: %w", err)
		}
	}
	return nil
}

func preflightSource(root *os.Root, name string) (map[copyFileID]struct{}, error) {
	result := make(map[copyFileID]struct{})
	var walk func(string) error
	walk = func(current string) error {
		info, err := root.Lstat(current)
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			linkName, err := root.Readlink(current)
			if err != nil {
				return err
			}
			if linkName == "" {
				return errors.New("symbolic link target is empty")
			}
			return nil
		case info.Mode().IsRegular():
			return nil
		case info.IsDir():
			id, err := copyID(info)
			if err != nil {
				return err
			}
			if _, exists := result[id]; exists {
				return errors.New("source directory contains a filesystem cycle")
			}
			result[id] = struct{}{}
			entries, err := readDirectory(root, current)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if err = walk(path.Join(current, entry.Name())); err != nil {
					return err
				}
			}
			return nil
		default:
			return fmt.Errorf("unsupported file type %s", info.Mode().Type())
		}
	}
	if err := walk(name); err != nil {
		return nil, err
	}
	return result, nil
}

func directoryIDExists(directories map[copyFileID]struct{}, info os.FileInfo) bool {
	id, err := copyID(info)
	if err != nil {
		return false
	}
	_, exists := directories[id]
	return exists
}

func copyID(info os.FileInfo) (copyFileID, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return copyFileID{}, errors.New("file identity is unavailable")
	}
	return copyFileID{device: uint64(stat.Dev), inode: uint64(stat.Ino)}, nil
}

func sameFile(left, right os.FileInfo) bool {
	leftID, leftErr := copyID(left)
	rightID, rightErr := copyID(right)
	return leftErr == nil && rightErr == nil && leftID == rightID
}

func preflightTargetAliases(root *os.Root, source, target string) error {
	sourceInfo, err := root.Lstat(source)
	if err != nil {
		return err
	}
	targetInfo, err := root.Lstat(target)
	if err != nil {
		return err
	}
	if sameFile(sourceInfo, targetInfo) {
		return errors.New("source and target are the same file")
	}
	if !sourceInfo.IsDir() || !targetInfo.IsDir() {
		return nil
	}
	entries, err := readDirectory(root, source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sourceChild := path.Join(source, entry.Name())
		targetChild := path.Join(target, entry.Name())
		_, err = root.Lstat(targetChild)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if err = preflightTargetAliases(root, sourceChild, targetChild); err != nil {
			return err
		}
	}
	return nil
}

func (self *CpCommand) copyEntry(root *os.Root, source, target string, overwrite bool) error {
	info, err := root.Lstat(source)
	if err != nil {
		return err
	}
	targetInfo, err := root.Lstat(target)
	targetExists := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if targetExists && !overwrite {
		return os.ErrExist
	}
	mergeDirectory := targetExists && info.IsDir() && targetInfo.IsDir()
	if mergeDirectory {
		entries, err := readDirectory(root, source)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err = self.copyEntry(root, path.Join(source, entry.Name()), path.Join(target, entry.Name()), overwrite); err != nil {
				return err
			}
		}
		return setMetadata(root, target, info)
	}
	return self.replaceEntry(root, source, target, targetExists)
}

func (self *CpCommand) copyEntryRaw(root *os.Root, source, target string, hardlinks map[copyFileID]string) error {
	info, err := root.Lstat(source)
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("file owner is unavailable")
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		linkName, err := root.Readlink(source)
		if err != nil {
			return err
		}
		if err = root.Symlink(linkName, target); err != nil {
			return err
		}
		return root.Lchown(target, int(stat.Uid), int(stat.Gid))
	case info.IsDir():
		if err = root.Mkdir(target, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := readDirectory(root, source)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err = self.copyEntryRaw(root, path.Join(source, entry.Name()), path.Join(target, entry.Name()), hardlinks); err != nil {
				return err
			}
		}
	case info.Mode().IsRegular():
		id, err := copyID(info)
		if err != nil {
			return err
		}
		if existing, ok := hardlinks[id]; ok {
			if err = root.Link(existing, target); err != nil {
				return err
			}
			return nil
		}
		sourceFile, err := root.Open(source)
		if err != nil {
			return err
		}
		targetFile, err := root.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
		if err != nil {
			_ = sourceFile.Close()
			return err
		}
		_, copyErr := io.Copy(targetFile, sourceFile)
		targetCloseErr := targetFile.Close()
		sourceCloseErr := sourceFile.Close()
		if copyErr != nil {
			return copyErr
		}
		if targetCloseErr != nil {
			return targetCloseErr
		}
		if sourceCloseErr != nil {
			return sourceCloseErr
		}
		hardlinks[id] = target
	default:
		return fmt.Errorf("unsupported file type %s", info.Mode().Type())
	}
	return setMetadata(root, target, info)
}

func setMetadata(root *os.Root, target string, info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("file owner is unavailable")
	}
	if err := root.Chown(target, int(stat.Uid), int(stat.Gid)); err != nil {
		return err
	}
	if err := root.Chmod(target, info.Mode()); err != nil {
		return err
	}
	return root.Chtimes(target, info.ModTime(), info.ModTime())
}

func (self *CpCommand) replaceEntry(root *os.Root, source, target string, targetExists bool) error {
	staging, err := unusedSibling(root, target, "copy")
	if err != nil {
		return err
	}
	if err = self.copyEntryRaw(root, source, staging, make(map[copyFileID]string)); err != nil {
		return errors.Join(err, root.RemoveAll(staging))
	}
	if !targetExists {
		if err = root.Rename(staging, target); err != nil {
			return errors.Join(err, root.RemoveAll(staging))
		}
		return nil
	}
	backup, err := unusedSibling(root, target, "backup")
	if err != nil {
		return errors.Join(err, root.RemoveAll(staging))
	}
	if err = root.Rename(target, backup); err != nil {
		return errors.Join(err, root.RemoveAll(staging))
	}
	if err = root.Rename(staging, target); err != nil {
		rollbackErr := root.Rename(backup, target)
		cleanupErr := root.RemoveAll(staging)
		if rollbackErr != nil {
			return fmt.Errorf("commit copy: %w; rollback failed and the original target remains at %q: %v", err, backup, rollbackErr)
		}
		return errors.Join(err, cleanupErr)
	}
	if err = root.RemoveAll(backup); err != nil {
		return fmt.Errorf("remove copy backup %q: %w", backup, err)
	}
	return nil
}

func unusedSibling(root *os.Root, target, purpose string) (string, error) {
	for range 16 {
		value := make([]byte, 8)
		if _, err := rand.Read(value); err != nil {
			return "", err
		}
		name := path.Join(path.Dir(target), ".dpanel-"+purpose+"-"+hex.EncodeToString(value))
		_, err := root.Lstat(name)
		if errors.Is(err, os.ErrNotExist) {
			return name, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", errors.New("cannot allocate a temporary copy path")
}
