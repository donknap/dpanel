package fs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
)

type Handler struct {
	ls      *LsCommand
	users   *UsersCommand
	du      *DuCommand
	cp      *CpCommand
	mv      *MvCommand
	mkdir   *MkdirCommand
	rm      *RmCommand
	chmod   *ChmodCommand
	chown   *ChownCommand
	chtimes *ChtimesCommand
}

type options struct {
	containerPID string
	root         string
	path         string
	source       string
	target       string
	mode         string
	uid          string
	gid          string
	atime        string
	mtime        string
	overwrite    bool
	recursive    bool
}

func New() *Handler {
	users := NewUsersCommand()
	cp := NewCpCommand()
	return &Handler{
		ls: NewLsCommand(users), users: users, du: NewDuCommand(), cp: cp, mv: NewMvCommand(cp),
		mkdir: NewMkdirCommand(), rm: NewRmCommand(), chmod: NewChmodCommand(), chown: NewChownCommand(),
		chtimes: NewChtimesCommand(),
	}
}

func newOptions(args []string) (options, error) {
	option := options{}
	values := make(map[string]string)
	for len(args) > 0 {
		name := args[0]
		args = args[1:]
		switch name {
		case "--overwrite":
			if option.overwrite {
				return option, fmt.Errorf("duplicate option: %s", name)
			}
			option.overwrite = true
			continue
		case "--recursive":
			if option.recursive {
				return option, fmt.Errorf("duplicate option: %s", name)
			}
			option.recursive = true
			continue
		case "--container-pid", "--root", "--path", "--source", "--target", "--mode", "--uid", "--gid", "--atime", "--mtime":
		default:
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
	option.containerPID = values["--container-pid"]
	option.root = values["--root"]
	option.path = values["--path"]
	option.source = values["--source"]
	option.target = values["--target"]
	option.mode = values["--mode"]
	option.uid = values["--uid"]
	option.gid = values["--gid"]
	option.atime = values["--atime"]
	option.mtime = values["--mtime"]
	return option, nil
}

func (self *Handler) Handle(_ context.Context, args []string) (any, error) {
	if len(args) == 0 {
		return nil, errors.New("fs operation is required")
	}
	option, err := newOptions(args[1:])
	if err != nil {
		return nil, err
	}
	if option.root == "" && option.containerPID == "" {
		return nil, errors.New("option --root or --container-pid is required")
	}
	if option.root != "" && option.containerPID != "" {
		return nil, errors.New("options --root and --container-pid are mutually exclusive")
	}
	rootPath := option.root
	if rootPath != "" {
		if strings.IndexByte(rootPath, 0) >= 0 || !path.IsAbs(rootPath) || path.Clean(rootPath) != rootPath {
			return nil, errors.New("invalid root path")
		}
		if rootPath == "/proc" || strings.HasPrefix(rootPath, "/proc/") {
			return nil, errors.New("option --root must not point to /proc")
		}
	} else {
		if runtime.GOOS != "linux" {
			return nil, errors.New("container filesystem access is only supported on Linux")
		}
		pid, err := strconv.ParseUint(option.containerPID, 10, 31)
		if err != nil || pid <= 1 || strconv.FormatUint(pid, 10) != option.containerPID {
			return nil, fmt.Errorf("invalid container pid %q", option.containerPID)
		}
		targetNamespace, err := os.Readlink(fmt.Sprintf("/proc/%d/ns/mnt", pid))
		if err != nil {
			return nil, fmt.Errorf("read target mount namespace: %w", err)
		}
		hostNamespace, err := os.Readlink("/proc/1/ns/mnt")
		if err != nil {
			return nil, fmt.Errorf("read host mount namespace: %w", err)
		}
		if targetNamespace == hostNamespace {
			return nil, errors.New("target shares the host mount namespace")
		}
		rootPath = fmt.Sprintf("/proc/%d/root", pid)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, fmt.Errorf("open root: %w", err)
	}
	defer root.Close()

	switch args[0] {
	case self.ls.Name:
		return self.ls.Run(root, option)
	case self.users.Name:
		return self.users.Run(root, option)
	case self.du.Name:
		return self.du.Run(root, option)
	case self.cp.Name:
		return self.cp.Run(root, option)
	case self.mv.Name:
		return self.mv.Run(root, option)
	case self.mkdir.Name:
		return self.mkdir.Run(root, option)
	case self.rm.Name:
		return self.rm.Run(root, option)
	case self.chmod.Name:
		return self.chmod.Run(root, option)
	case self.chown.Name:
		return self.chown.Run(root, option)
	case self.chtimes.Name:
		return self.chtimes.Run(root, option)
	default:
		return nil, fmt.Errorf("unknown fs operation: %s", args[0])
	}
}
