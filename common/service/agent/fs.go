package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	agentTypes "github.com/donknap/dpanel/app/agent/types"
)

type FsTarget struct {
	Root         string
	ContainerPID int
}

type Fs struct {
	agent *Agent
}

func (self *Fs) Mkdir(ctx context.Context, target FsTarget, path string, mode os.FileMode, recursive bool) error {
	args := []string{"fs", "mkdir", "--path", path, "--mode", formatFsMode(mode)}
	if recursive {
		args = append(args, "--recursive")
	}
	return self.exec(ctx, target, nil, args...)
}

func (self *Fs) Remove(ctx context.Context, target FsTarget, path string, recursive bool) error {
	args := []string{"fs", "rm", "--path", path}
	if recursive {
		args = append(args, "--recursive")
	}
	return self.exec(ctx, target, nil, args...)
}

func (self *Fs) Chmod(ctx context.Context, target FsTarget, path string, mode os.FileMode, recursive bool) error {
	args := []string{"fs", "chmod", "--path", path, "--mode", formatFsMode(mode)}
	if recursive {
		args = append(args, "--recursive")
	}
	return self.exec(ctx, target, nil, args...)
}

func (self *Fs) Chown(ctx context.Context, target FsTarget, path string, uid, gid *int, recursive bool) error {
	args := []string{"fs", "chown", "--path", path}
	if uid != nil && *uid >= 0 {
		args = append(args, "--uid", strconv.Itoa(*uid))
	}
	if gid != nil && *gid >= 0 {
		args = append(args, "--gid", strconv.Itoa(*gid))
	}
	if len(args) == 4 {
		return errors.New("chown requires uid or gid")
	}
	if recursive {
		args = append(args, "--recursive")
	}
	return self.exec(ctx, target, nil, args...)
}

func (self *Fs) Chtimes(ctx context.Context, target FsTarget, path string, atime, mtime time.Time) error {
	return self.exec(ctx, target, nil,
		"fs", "chtimes", "--path", path,
		"--atime", atime.Format(time.RFC3339Nano),
		"--mtime", mtime.Format(time.RFC3339Nano),
	)
}

func (self *Fs) List(ctx context.Context, target FsTarget, path string) ([]agentTypes.FileData, error) {
	result := make([]agentTypes.FileData, 0)
	if err := self.exec(ctx, target, &result, "fs", "ls", "--path", path); err != nil {
		return nil, err
	}
	return result, nil
}

func (self *Fs) Size(ctx context.Context, target FsTarget, path string) (agentTypes.SizeData, error) {
	result := agentTypes.SizeData{}
	if err := self.exec(ctx, target, &result, "fs", "du", "--path", path); err != nil {
		return agentTypes.SizeData{}, err
	}
	return result, nil
}

func (self *Fs) Users(ctx context.Context, target FsTarget) (agentTypes.IdentityList, error) {
	result := agentTypes.IdentityList{}
	if err := self.exec(ctx, target, &result, "fs", "users"); err != nil {
		return agentTypes.IdentityList{}, err
	}
	return result, nil
}

func (self *Fs) Copy(ctx context.Context, target FsTarget, source, destination string, overwrite bool) error {
	return self.transfer(ctx, target, "cp", source, destination, overwrite)
}

func (self *Fs) Move(ctx context.Context, target FsTarget, source, destination string, overwrite bool) error {
	return self.transfer(ctx, target, "mv", source, destination, overwrite)
}

func (self *Fs) transfer(
	ctx context.Context, target FsTarget, command, source, destination string, overwrite bool,
) error {
	args := []string{"fs", command, "--source", source, "--target", destination}
	if overwrite {
		args = append(args, "--overwrite")
	}
	return self.exec(ctx, target, nil, args...)
}

func (self *Fs) exec(ctx context.Context, target FsTarget, result any, args ...string) error {
	targetArgs, err := target.args()
	if err != nil {
		return err
	}
	return self.agent.exec(ctx, result, append(args, targetArgs...)...)
}

func (self FsTarget) args() ([]string, error) {
	if self.Root != "" && self.ContainerPID != 0 {
		return nil, errors.New("agent filesystem target root and container pid are mutually exclusive")
	}
	if self.Root != "" {
		return []string{"--root", self.Root}, nil
	}
	if self.ContainerPID <= 1 {
		return nil, errors.New("agent filesystem target root or container pid is required")
	}
	return []string{"--container-pid", strconv.Itoa(self.ContainerPID)}, nil
}

func formatFsMode(mode os.FileMode) string {
	value := uint32(mode.Perm())
	if mode&os.ModeSetuid != 0 {
		value |= 0o4000
	}
	if mode&os.ModeSetgid != 0 {
		value |= 0o2000
	}
	if mode&os.ModeSticky != 0 {
		value |= 0o1000
	}
	return fmt.Sprintf("%04o", value)
}
