package fs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

type Handler struct{}

type fileData struct {
	Name     string      `json:"name"`
	Size     int64       `json:"size"`
	Mode     os.FileMode `json:"mode"`
	ModeText string      `json:"modeText"`
	ModTime  time.Time   `json:"modTime"`
	UID      uint32      `json:"uid"`
	GID      uint32      `json:"gid"`
	User     string      `json:"user"`
	Group    string      `json:"group"`
	LinkName string      `json:"linkName"`
}

type userIdentity struct {
	Name        string `json:"name"`
	UID         uint32 `json:"uid"`
	GID         uint32 `json:"gid"`
	Description string `json:"description"`
}

type groupIdentity struct {
	Name string `json:"name"`
	GID  uint32 `json:"gid"`
}

type identityList struct {
	User  []userIdentity  `json:"user"`
	Group []groupIdentity `json:"group"`
}

type options struct {
	root      string
	path      string
	source    string
	target    string
	mode      os.FileMode
	recursive bool
}

type fileID struct {
	device uint64
	inode  uint64
}

func New() *Handler {
	return &Handler{}
}

func (*Handler) Handle(_ context.Context, args []string) (any, error) {
	if len(args) == 0 {
		return nil, errors.New("fs operation is required")
	}

	operation := args[0]
	option, err := parseOptions(operation, args[1:])
	if err != nil {
		return nil, err
	}

	root, err := os.OpenRoot(option.root)
	if err != nil {
		return nil, fmt.Errorf("open root: %w", err)
	}
	defer root.Close()

	switch operation {
	case "list":
		return list(root, relativePath(option.path))
	case "identities":
		return readIdentities(root), nil
	case "size":
		size, err := directorySize(root, relativePath(option.path))
		if err != nil {
			return nil, err
		}
		return map[string]int64{"size": size}, nil
	case "copy":
		if err = copyPath(root, relativePath(option.source), relativePath(option.target)); err != nil {
			return nil, fmt.Errorf("copy %q to %q: %w", option.source, option.target, err)
		}
		return nil, nil
	case "move":
		if err = movePath(root, relativePath(option.source), relativePath(option.target)); err != nil {
			return nil, fmt.Errorf("move %q to %q: %w", option.source, option.target, err)
		}
		return nil, nil
	case "mkdir-all":
		relativePath := relativePath(option.path)
		_, err = root.Lstat(relativePath)
		created := errors.Is(err, os.ErrNotExist)
		if err != nil && !created {
			return nil, fmt.Errorf("lstat directory %q: %w", option.path, err)
		}
		if err = root.MkdirAll(relativePath, option.mode.Perm()); err != nil {
			return nil, fmt.Errorf("mkdir-all %q: %w", option.path, err)
		}
		if created {
			err = root.Chmod(relativePath, option.mode)
		}
		if err != nil {
			return nil, fmt.Errorf("chmod created directory %q: %w", option.path, err)
		}
		return nil, nil
	case "remove-all":
		if option.path == "/" {
			return nil, errors.New("refusing to remove root directory")
		}
		if err = root.RemoveAll(relativePath(option.path)); err != nil {
			return nil, fmt.Errorf("remove-all %q: %w", option.path, err)
		}
		return nil, nil
	case "chmod":
		if option.recursive && option.path == "/" {
			return nil, errors.New("refusing to recursively chmod root directory")
		}
		relativePath := relativePath(option.path)
		if option.recursive {
			err = chmodRecursive(root, relativePath, option.mode)
		} else {
			err = chmodOne(root, relativePath, option.mode)
		}
		if err != nil {
			return nil, fmt.Errorf("chmod %q: %w", option.path, err)
		}
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown fs operation: %s", operation)
	}
}

func parseOptions(operation string, args []string) (options, error) {
	option := options{}
	values := make(map[string]string)
	for len(args) > 0 {
		name := args[0]
		args = args[1:]
		if name == "--recursive" {
			if option.recursive {
				return option, errors.New("duplicate option: --recursive")
			}
			option.recursive = true
			continue
		}
		if name != "--root" && name != "--path" && name != "--source" && name != "--target" && name != "--mode" {
			return option, fmt.Errorf("unknown option: %s", name)
		}
		if _, ok := values[name]; ok {
			return option, fmt.Errorf("duplicate option: %s", name)
		}
		if len(args) == 0 {
			return option, fmt.Errorf("option %s requires a value", name)
		}
		values[name] = args[0]
		args = args[1:]
	}

	if operation != "list" && operation != "identities" && operation != "size" && operation != "copy" && operation != "move" && operation != "mkdir-all" && operation != "remove-all" && operation != "chmod" {
		return option, fmt.Errorf("unknown fs operation: %s", operation)
	}
	if err := validateAbsolutePath("root", values["--root"]); err != nil {
		return option, err
	}
	needsPath := operation != "identities" && operation != "copy" && operation != "move"
	if needsPath {
		if err := validateAbsolutePath("path", values["--path"]); err != nil {
			return option, err
		}
	} else if _, ok := values["--path"]; ok {
		return option, fmt.Errorf("operation %s does not accept --path", operation)
	}
	needsTransferPaths := operation == "copy" || operation == "move"
	if needsTransferPaths {
		if err := validateAbsolutePath("source", values["--source"]); err != nil {
			return option, err
		}
		if err := validateAbsolutePath("target", values["--target"]); err != nil {
			return option, err
		}
	} else if _, sourceOk := values["--source"]; sourceOk {
		return option, fmt.Errorf("operation %s does not accept --source", operation)
	} else if _, targetOk := values["--target"]; targetOk {
		return option, fmt.Errorf("operation %s does not accept --target", operation)
	}
	option.root = values["--root"]
	option.path = values["--path"]
	option.source = values["--source"]
	option.target = values["--target"]

	needsMode := operation == "mkdir-all" || operation == "chmod"
	modeValue, hasMode := values["--mode"]
	if needsMode != hasMode {
		if needsMode {
			return option, errors.New("option --mode is required")
		}
		return option, fmt.Errorf("operation %s does not accept --mode", operation)
	}
	if option.recursive && operation != "chmod" {
		return option, fmt.Errorf("operation %s does not accept --recursive", operation)
	}
	if hasMode {
		mode, err := parseMode(modeValue)
		if err != nil {
			return option, err
		}
		option.mode = mode
	}

	return option, nil
}

func relativePath(value string) string {
	value = strings.TrimPrefix(value, "/")
	if value == "" {
		return "."
	}
	return value
}

func validateAbsolutePath(name, value string) error {
	if value == "" {
		return fmt.Errorf("option --%s is required", name)
	}
	if strings.IndexByte(value, 0) >= 0 {
		return fmt.Errorf("%s contains a null byte", name)
	}
	if !strings.HasPrefix(value, "/") {
		return fmt.Errorf("%s must be absolute", name)
	}
	if path.Clean(value) != value {
		return fmt.Errorf("%s must be canonical", name)
	}
	return nil
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

func list(root *os.Root, relativePath string) ([]fileData, error) {
	directory, err := root.Open(relativePath)
	if err != nil {
		return nil, fmt.Errorf("open directory: %w", err)
	}
	defer directory.Close()

	entries, err := directory.ReadDir(-1)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}
	identities := readIdentities(root)
	users := make(map[uint32]string, len(identities.User)+1)
	groups := make(map[uint32]string, len(identities.Group)+1)
	for _, user := range identities.User {
		if _, exists := users[user.UID]; !exists {
			users[user.UID] = user.Name
		}
	}
	for _, group := range identities.Group {
		if _, exists := groups[group.GID]; !exists {
			groups[group.GID] = group.Name
		}
	}
	if _, exists := users[0]; !exists {
		users[0] = "root"
	}
	if _, exists := groups[0]; !exists {
		groups[0] = "root"
	}
	result := make([]fileData, 0, len(entries))
	for _, entry := range entries {
		if !utf8.ValidString(entry.Name()) {
			return nil, fmt.Errorf("directory contains a non-UTF-8 filename: %q", []byte(entry.Name()))
		}
		entryPath := path.Join(relativePath, entry.Name())
		info, err := root.Lstat(entryPath)
		if err != nil {
			return nil, fmt.Errorf("lstat %q: %w", entry.Name(), err)
		}
		uid, gid, err := owner(info)
		if err != nil {
			return nil, fmt.Errorf("read owner for %q: %w", entry.Name(), err)
		}
		item := fileData{
			Name:     entry.Name(),
			Size:     info.Size(),
			Mode:     info.Mode(),
			ModeText: info.Mode().String(),
			ModTime:  info.ModTime(),
			UID:      uid,
			GID:      gid,
			User:     identityName(users, uid),
			Group:    identityName(groups, gid),
		}
		if info.Mode()&os.ModeSymlink != 0 {
			item.LinkName, err = root.Readlink(entryPath)
			if err != nil {
				return nil, fmt.Errorf("readlink %q: %w", entry.Name(), err)
			}
			if !utf8.ValidString(item.LinkName) {
				return nil, fmt.Errorf("symlink %q has a non-UTF-8 target", entry.Name())
			}
		}
		result = append(result, item)
	}
	return result, nil
}

func readIdentities(root *os.Root) identityList {
	result := identityList{
		User:  make([]userIdentity, 0),
		Group: make([]groupIdentity, 0),
	}
	if content, err := root.ReadFile("etc/passwd"); err == nil {
		for _, line := range strings.Split(string(content), "\n") {
			fields := strings.Split(line, ":")
			if len(fields) < 7 || fields[0] == "" {
				continue
			}
			uid, uidErr := strconv.ParseUint(fields[2], 10, 32)
			gid, gidErr := strconv.ParseUint(fields[3], 10, 32)
			if uidErr != nil || gidErr != nil {
				continue
			}
			result.User = append(result.User, userIdentity{
				Name:        fields[0],
				UID:         uint32(uid),
				GID:         uint32(gid),
				Description: fields[4],
			})
		}
	}
	if content, err := root.ReadFile("etc/group"); err == nil {
		for _, line := range strings.Split(string(content), "\n") {
			fields := strings.Split(line, ":")
			if len(fields) < 3 || fields[0] == "" {
				continue
			}
			gid, err := strconv.ParseUint(fields[2], 10, 32)
			if err != nil {
				continue
			}
			result.Group = append(result.Group, groupIdentity{
				Name: fields[0],
				GID:  uint32(gid),
			})
		}
	}
	return result
}

func identityName(identities map[uint32]string, id uint32) string {
	if name, ok := identities[id]; ok {
		return name
	}
	return strconv.FormatUint(uint64(id), 10)
}

func owner(info os.FileInfo) (uint32, uint32, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, errors.New("file owner is unavailable")
	}
	return stat.Uid, stat.Gid, nil
}

func directorySize(root *os.Root, relativePath string) (int64, error) {
	info, err := root.Lstat(relativePath)
	if err != nil {
		return 0, fmt.Errorf("lstat directory: %w", err)
	}
	if !info.IsDir() {
		return 0, errors.New("target is not a directory")
	}

	seen := make(map[fileID]struct{})
	var total int64
	var walk func(string) error
	walk = func(directoryPath string) error {
		directory, err := root.Open(directoryPath)
		if err != nil {
			return fmt.Errorf("open directory %q: %w", directoryPath, err)
		}
		entries, readErr := directory.ReadDir(-1)
		closeErr := directory.Close()
		if readErr != nil {
			return fmt.Errorf("read directory %q: %w", directoryPath, readErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close directory %q: %w", directoryPath, closeErr)
		}
		for _, entry := range entries {
			entryPath := path.Join(directoryPath, entry.Name())
			info, err := root.Lstat(entryPath)
			if err != nil {
				return fmt.Errorf("lstat %q: %w", entryPath, err)
			}
			switch {
			case info.IsDir():
				if err = walk(entryPath); err != nil {
					return err
				}
			case info.Mode().IsRegular():
				id, err := inode(info)
				if err != nil {
					return fmt.Errorf("read inode for %q: %w", entryPath, err)
				}
				if _, ok := seen[id]; !ok {
					seen[id] = struct{}{}
					total += info.Size()
				}
			}
		}
		return nil
	}
	if err = walk(relativePath); err != nil {
		return 0, err
	}
	return total, nil
}

func inode(info os.FileInfo) (fileID, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fileID{}, errors.New("inode is unavailable")
	}
	return fileID{device: uint64(stat.Dev), inode: uint64(stat.Ino)}, nil
}

func copyPath(root *os.Root, source, target string) (err error) {
	if source == "." || target == "." {
		return errors.New("root directory cannot be copied")
	}
	if source == target {
		return errors.New("source and target are the same")
	}
	sourceInfo, err := root.Lstat(source)
	if err != nil {
		return fmt.Errorf("lstat source: %w", err)
	}
	if sourceInfo.IsDir() && strings.HasPrefix(target+"/", source+"/") {
		return errors.New("a directory cannot be copied into itself")
	}
	if _, err = root.Lstat(target); err == nil {
		return os.ErrExist
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("lstat target: %w", err)
	}
	parentInfo, err := root.Stat(path.Dir(target))
	if err != nil {
		return fmt.Errorf("stat target parent: %w", err)
	}
	if !parentInfo.IsDir() {
		return errors.New("target parent is not a directory")
	}

	defer func() {
		if err != nil {
			_ = root.RemoveAll(target)
		}
	}()
	return copyEntry(root, source, target, make(map[fileID]string))
}

func copyEntry(root *os.Root, source, target string, hardlinks map[fileID]string) error {
	info, err := root.Lstat(source)
	if err != nil {
		return err
	}
	uid, gid, err := owner(info)
	if err != nil {
		return err
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
		return root.Lchown(target, int(uid), int(gid))
	case info.IsDir():
		if err = root.Mkdir(target, info.Mode().Perm()); err != nil {
			return err
		}
		directory, err := root.Open(source)
		if err != nil {
			return err
		}
		entries, readErr := directory.ReadDir(-1)
		closeErr := directory.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		for _, entry := range entries {
			if err = copyEntry(root, path.Join(source, entry.Name()), path.Join(target, entry.Name()), hardlinks); err != nil {
				return err
			}
		}
	case info.Mode().IsRegular():
		id, err := inode(info)
		if err != nil {
			return err
		}
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

	if err = root.Chown(target, int(uid), int(gid)); err != nil {
		return err
	}
	if err = root.Chmod(target, info.Mode()); err != nil {
		return err
	}
	return root.Chtimes(target, info.ModTime(), info.ModTime())
}

func movePath(root *os.Root, source, target string) error {
	if source == "." || target == "." {
		return errors.New("root directory cannot be moved")
	}
	if source == target {
		return errors.New("source and target are the same")
	}
	if _, err := root.Lstat(target); err == nil {
		return os.ErrExist
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("lstat target: %w", err)
	}
	if err := root.Rename(source, target); err == nil {
		return nil
	} else if !errors.Is(err, syscall.EXDEV) {
		return err
	}
	if err := copyPath(root, source, target); err != nil {
		return err
	}
	if err := root.RemoveAll(source); err != nil {
		return fmt.Errorf("remove source after cross-device copy: %w", err)
	}
	return nil
}

func chmodRecursive(root *os.Root, relativePath string, mode os.FileMode) error {
	info, err := root.Lstat(relativePath)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	if info.IsDir() {
		directory, err := root.Open(relativePath)
		if err != nil {
			return err
		}
		entries, readErr := directory.ReadDir(-1)
		closeErr := directory.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		for _, entry := range entries {
			if err = chmodRecursive(root, path.Join(relativePath, entry.Name()), mode); err != nil {
				return err
			}
		}
	}
	return root.Chmod(relativePath, mode)
}

func chmodOne(root *os.Root, relativePath string, mode os.FileMode) error {
	info, err := root.Lstat(relativePath)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("refusing to chmod a symbolic link")
	}
	return root.Chmod(relativePath, mode)
}
