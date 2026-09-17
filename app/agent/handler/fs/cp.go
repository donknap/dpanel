package fs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
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
	if sourceInfo.IsDir() && strings.HasPrefix(target+"/", source+"/") {
		return errors.New("a directory cannot be copied or moved into itself")
	}
	_, err = root.Lstat(target)
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
	if err = self.copyEntry(root, source, target, overwrite, make(map[copyFileID]string)); err != nil {
		return err
	}
	if move {
		if err = root.RemoveAll(source); err != nil {
			return fmt.Errorf("remove source after copy: %w", err)
		}
	}
	return nil
}

func (self *CpCommand) copyEntry(root *os.Root, source, target string, overwrite bool, hardlinks map[copyFileID]string) error {
	info, err := root.Lstat(source)
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("file owner is unavailable")
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
	if targetExists && !mergeDirectory {
		if err = root.RemoveAll(target); err != nil {
			return err
		}
		targetExists = false
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
		if !targetExists {
			if err = root.Mkdir(target, info.Mode().Perm()); err != nil {
				return err
			}
		}
		entries, err := readDirectory(root, source)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err = self.copyEntry(root, path.Join(source, entry.Name()), path.Join(target, entry.Name()), overwrite, hardlinks); err != nil {
				return err
			}
		}
	case info.Mode().IsRegular():
		id := copyFileID{device: uint64(stat.Dev), inode: uint64(stat.Ino)}
		if existing, ok := hardlinks[id]; ok {
			return root.Link(existing, target)
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
	if err = root.Chown(target, int(stat.Uid), int(stat.Gid)); err != nil {
		return err
	}
	if err = root.Chmod(target, info.Mode()); err != nil {
		return err
	}
	return root.Chtimes(target, info.ModTime(), info.ModTime())
}
