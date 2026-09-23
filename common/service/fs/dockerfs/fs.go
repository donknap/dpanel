package dockerfs

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/errdefs"
	serviceAgent "github.com/donknap/dpanel/common/service/agent"
	archiveservice "github.com/donknap/dpanel/common/service/archive"
	"github.com/donknap/dpanel/common/service/docker"
	serviceafs "github.com/donknap/dpanel/common/service/fs/afs"
	"github.com/donknap/dpanel/common/service/fs/tempfs"
	fsdata "github.com/donknap/dpanel/common/types/fs"
	"github.com/spf13/afero"
)

var _ serviceafs.Fs = (*Fs)(nil)

const maxSymlinkDepth = 40

type Fs struct {
	name                string
	sdk                 *docker.Client
	agent               *serviceAgent.Agent
	targetContainerName string
	proxyContainerName  string
	rootPath            string
	workingDir          string
	targetType          targetType
	destroy             func() error
}

type targetType uint8

const (
	targetTypeContainer targetType = iota + 1
	targetTypeMount
)

func New(opts ...Option) (*Fs, error) {
	o := &Fs{}
	for _, opt := range opts {
		if err := opt(o); err != nil {
			return nil, err
		}
	}
	if o.sdk == nil {
		return nil, errors.New("invalid docker sdk")
	}
	if o.targetType == targetTypeMount {
		o.targetContainerName = o.proxyContainerName
	}
	if o.proxyContainerName == "" || o.targetContainerName == "" || o.targetType == 0 {
		return nil, fmt.Errorf("the %s container does not exist or is not running", o.targetContainerName)
	}
	agentClient, err := serviceAgent.NewDockerAgent(o.sdk, o.proxyContainerName)
	if err != nil {
		return nil, err
	}
	o.agent = agentClient
	if o.rootPath == "" || strings.IndexByte(o.rootPath, 0) >= 0 || !path.IsAbs(o.rootPath) || path.Clean(o.rootPath) != o.rootPath {
		return nil, errors.New("invalid backend root")
	}
	if o.workingDir == "" {
		o.workingDir = "/"
	}
	if _, err := o.pathName(o.workingDir); err != nil {
		return nil, fmt.Errorf("invalid working directory: %w", err)
	}
	return o, nil
}

func (self *Fs) Name() string {
	return self.name
}

func (self *Fs) Destroy() error {
	if self.destroy == nil {
		return nil
	}
	return self.destroy()
}

func (self *Fs) Create(name string) (afero.File, error) {
	return self.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o666)
}

func (self *Fs) Mkdir(name string, perm os.FileMode) error {
	p, err := self.pathName(name)
	if err != nil {
		return err
	}
	target, err := self.agentTarget()
	if err != nil {
		return err
	}
	return self.agent.Fs.Mkdir(self.sdk.Ctx, target, p, perm, false)
}

func (self *Fs) MkdirAll(name string, perm os.FileMode) error {
	p, err := self.pathName(name)
	if err != nil {
		return err
	}
	target, err := self.agentTarget()
	if err != nil {
		return err
	}
	return self.agent.Fs.Mkdir(self.sdk.Ctx, target, p, perm, true)
}

func (self *Fs) Open(name string) (afero.File, error) {
	return self.OpenFile(name, os.O_RDONLY, 0)
}

func (self *Fs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	cleanName, err := self.pathName(name)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: name, Err: err}
	}
	accessMode := flag & (os.O_WRONLY | os.O_RDWR)
	if accessMode == os.O_WRONLY|os.O_RDWR {
		return nil, &os.PathError{Op: "open", Path: name, Err: os.ErrInvalid}
	}
	readable := accessMode != os.O_WRONLY
	writable := accessMode == os.O_WRONLY || accessMode == os.O_RDWR
	if flag&os.O_TRUNC != 0 && !writable {
		return nil, &os.PathError{Op: "open", Path: name, Err: os.ErrPermission}
	}
	if flag&os.O_APPEND != 0 {
		return nil, &os.PathError{Op: "open", Path: name, Err: errors.New("append mode is not supported")}
	}

	resolvedBackendName, pathStat, initialExists, err := self.resolvePath(cleanName)
	exists := err == nil
	if err != nil && !isNotExist(err) {
		return nil, err
	}
	if flag&os.O_CREATE != 0 && flag&os.O_EXCL != 0 && initialExists {
		return nil, &os.PathError{Op: "open", Path: name, Err: os.ErrExist}
	}
	if !exists && flag&os.O_CREATE == 0 {
		return nil, err
	}
	if exists && pathStat.Mode.IsDir() {
		if writable || flag&(os.O_CREATE|os.O_TRUNC|os.O_APPEND) != 0 {
			return nil, &os.PathError{Op: "open", Path: name, Err: errors.New("is a directory")}
		}
		resolvedName, err := self.virtualPath(resolvedBackendName)
		if err != nil {
			return nil, err
		}
		return &File{fs: self, name: resolvedName, info: self.fileInfo(resolvedName, pathStat), readable: true, directory: true}, nil
	}
	if exists && !pathStat.Mode.IsRegular() {
		return nil, &os.PathError{Op: "open", Path: name, Err: fmt.Errorf("unsupported file type %s", pathStat.Mode.Type())}
	}

	resolvedName, err := self.virtualPath(resolvedBackendName)
	if err != nil {
		return nil, err
	}
	if !exists {
		pathStat = container.PathStat{Name: path.Base(resolvedBackendName), Mode: perm.Perm()}
	}
	if !exists || flag&os.O_TRUNC != 0 {
		pathStat.Size = 0
		pathStat.Mtime = time.Now()
	}
	file := &File{
		fs: self, name: resolvedName, backendName: resolvedBackendName,
		info: self.fileInfo(resolvedName, pathStat), readable: readable, writable: writable,
		syncMode: flag&os.O_SYNC != 0, loadContent: exists && flag&os.O_TRUNC == 0,
	}
	if !exists || flag&os.O_TRUNC != 0 {
		file.dirty = true
	}
	return file, nil
}

func (self *Fs) Remove(name string) error {
	p, err := self.pathName(name)
	if err != nil {
		return err
	}
	target, err := self.agentTarget()
	if err != nil {
		return err
	}
	return self.agent.Fs.Remove(self.sdk.Ctx, target, p, false)
}

func (self *Fs) RemoveAll(name string) error {
	p, err := self.pathName(name)
	if err != nil {
		return err
	}
	if p == "/" {
		return &os.PathError{Op: "remove_all", Path: p, Err: errors.New("refusing to remove root directory")}
	}
	target, err := self.agentTarget()
	if err != nil {
		return err
	}
	return self.agent.Fs.Remove(self.sdk.Ctx, target, p, true)
}

func (self *Fs) Rename(oldname, newname string) error {
	return self.Move(oldname, newname, false)
}

func (self *Fs) Stat(name string) (os.FileInfo, error) {
	cleanName, err := self.pathName(name)
	if err != nil {
		return nil, &os.PathError{Op: "stat", Path: name, Err: err}
	}
	resolvedName, pathStat, _, err := self.resolvePath(cleanName)
	if err != nil {
		return nil, err
	}
	resolvedName, err = self.virtualPath(resolvedName)
	if err != nil {
		return nil, err
	}
	return self.fileInfo(resolvedName, pathStat), nil
}

func (self *Fs) LstatIfPossible(name string) (os.FileInfo, bool, error) {
	cleanName, err := self.pathName(name)
	if err != nil {
		return nil, true, &os.PathError{Op: "lstat", Path: name, Err: err}
	}
	parentBackendName, _, _, err := self.resolvePath(path.Dir(cleanName))
	if err != nil {
		return nil, true, err
	}
	backendName := parentBackendName
	if cleanName != "/" {
		backendName = path.Join(parentBackendName, path.Base(cleanName))
	}
	pathStat, err := self.sdk.Client.ContainerStatPath(self.sdk.Ctx, self.targetContainerName, backendName)
	if err != nil {
		return nil, true, err
	}
	return self.fileInfo(cleanName, pathStat), true, nil
}

func (self *Fs) ReadlinkIfPossible(name string) (string, error) {
	info, _, err := self.LstatIfPossible(name)
	if err != nil {
		return "", err
	}
	data, ok := info.Sys().(*fsdata.FileData)
	if !ok || info.Mode()&os.ModeSymlink == 0 || data.LinkName == "" {
		return "", afero.ErrNoReadlink
	}
	if path.IsAbs(data.LinkName) && self.rootPath != "/" {
		return self.virtualPath(path.Clean(data.LinkName))
	}
	return data.LinkName, nil
}

func (self *Fs) Chmod(name string, mode os.FileMode) error {
	return self.chmod(name, mode, false)
}

func (self *Fs) ChmodAll(name string, mode os.FileMode, recursive bool) error {
	return self.chmod(name, mode, recursive)
}

func (self *Fs) chmod(name string, mode os.FileMode, recursive bool) error {
	p, err := self.pathName(name)
	if err != nil {
		return err
	}
	if recursive && p == "/" {
		return &os.PathError{Op: "chmod", Path: p, Err: errors.New("refusing to recursively chmod root directory")}
	}
	target, err := self.agentTarget()
	if err != nil {
		return err
	}
	return self.agent.Fs.Chmod(self.sdk.Ctx, target, p, mode, recursive)
}

func (self *Fs) Chown(name string, uid, gid int) error {
	return self.chown(name, &uid, &gid, false)
}

func (self *Fs) ChownAll(name string, uid, gid *int, recursive bool) error {
	return self.chown(name, uid, gid, recursive)
}

func (self *Fs) chown(name string, uid, gid *int, recursive bool) error {
	p, err := self.pathName(name)
	if err != nil {
		return err
	}
	if recursive && p == "/" {
		return &os.PathError{Op: "chown", Path: p, Err: errors.New("refusing to recursively chown root directory")}
	}
	target, err := self.agentTarget()
	if err != nil {
		return err
	}
	return self.agent.Fs.Chown(self.sdk.Ctx, target, p, uid, gid, recursive)
}

func (self *Fs) Chtimes(name string, atime, mtime time.Time) error {
	p, err := self.pathName(name)
	if err != nil {
		return err
	}
	target, err := self.agentTarget()
	if err != nil {
		return err
	}
	return self.agent.Fs.Chtimes(self.sdk.Ctx, target, p, atime, mtime)
}

func (self *Fs) readDirFromContainer(rootPath string) ([]os.FileInfo, error) {
	agentPath, err := self.pathName(rootPath)
	if err != nil {
		return nil, err
	}
	target, err := self.agentTarget()
	if err != nil {
		return nil, err
	}
	result, err := self.agent.Fs.List(self.sdk.Ctx, target, agentPath)
	if err != nil {
		return nil, err
	}
	fileList := make([]os.FileInfo, 0, len(result))
	for _, item := range result {
		fileData := &fsdata.FileData{
			Path: path.Join(rootPath, item.Name), Name: item.Name, Size: item.Size,
			UID: item.UID, GID: item.GID, Mod: item.Mode, ModStr: item.ModeText,
			Change: fsdata.ChangeDefault, ModTime: item.ModTime, User: item.User,
			Group: item.Group, LinkName: item.LinkName, IsDir: item.Mode.IsDir(),
			IsSymlink: item.Mode&os.ModeSymlink != 0,
		}
		fileList = append(fileList, fsdata.NewFileInfo(fileData))
	}
	return fileList, nil
}

func (self *Fs) ReadDir(name string) ([]os.FileInfo, error) {
	fileList, err := self.readDirFromContainer(name)
	if err != nil || self.targetType != targetTypeContainer {
		return fileList, err
	}
	changeList, err := self.sdk.Client.ContainerDiff(self.sdk.Ctx, self.targetContainerName)
	if err != nil {
		return fileList, nil
	}
	changes := make(map[string]container.FilesystemChange, len(changeList))
	for _, change := range changeList {
		changes[change.Path] = change
	}
	for _, info := range fileList {
		data, ok := info.Sys().(*fsdata.FileData)
		if !ok {
			continue
		}
		change, ok := changes[data.Path]
		if !ok {
			continue
		}
		switch int(change.Kind) {
		case 0:
			data.Change = fsdata.ChangeModified
		case 1:
			data.Change = fsdata.ChangeAdd
		case 2:
			data.Change = fsdata.ChangeDeleted
		}
	}
	return fileList, nil
}

func (self *Fs) List(name string) ([]*fsdata.FileData, error) {
	entries, err := self.ReadDir(name)
	if err != nil {
		return nil, err
	}
	result := make([]*fsdata.FileData, 0, len(entries))
	for _, entry := range entries {
		data, ok := entry.Sys().(*fsdata.FileData)
		if !ok {
			return nil, errors.New("docker filesystem returned invalid file metadata")
		}
		clone := *data
		result = append(result, &clone)
	}
	if self.targetType != targetTypeContainer {
		return result, nil
	}
	containerInfo, err := self.sdk.Client.ContainerInspect(self.sdk.Ctx, self.targetContainerName)
	if err != nil {
		return nil, err
	}
	for _, item := range result {
		for _, mount := range containerInfo.Mounts {
			if item.Path == mount.Destination || strings.HasPrefix(item.Path, strings.TrimSuffix(mount.Destination, "/")+"/") {
				item.Change = fsdata.ChangeVolume
				break
			}
		}
	}
	return result, nil
}

func (self *Fs) Info(name string) (*fsdata.FileData, error) {
	info, err := self.Stat(name)
	if err != nil {
		return nil, err
	}
	data, ok := info.Sys().(*fsdata.FileData)
	if !ok {
		return nil, errors.New("docker filesystem returned invalid file metadata")
	}
	clone := *data
	return &clone, nil
}

func (self *Fs) WorkingDir() string {
	return self.workingDir
}

func (self *Fs) RootDirs() ([]string, error) {
	if self.targetType != targetTypeContainer {
		return nil, nil
	}
	containerInfo, err := self.sdk.Client.ContainerInspect(self.sdk.Ctx, self.targetContainerName)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(containerInfo.Mounts)+1)
	if containerInfo.Config != nil && containerInfo.Config.WorkingDir != "" {
		result = append(result, containerInfo.Config.WorkingDir)
	}
	for _, item := range containerInfo.Mounts {
		pathStat, statErr := self.sdk.Client.ContainerStatPath(self.sdk.Ctx, containerInfo.ID, item.Destination)
		if statErr == nil && pathStat.Mode.IsDir() {
			result = append(result, item.Destination)
		}
	}
	return result, nil
}

func (self *Fs) PathSize(name string) (int64, error) {
	info, err := self.Stat(name)
	if err != nil {
		return 0, err
	}
	if !info.IsDir() {
		return info.Size(), nil
	}
	p, err := self.pathName(name)
	if err != nil {
		return 0, err
	}
	target, err := self.agentTarget()
	if err != nil {
		return 0, err
	}
	result, err := self.agent.Fs.Size(self.sdk.Ctx, target, p)
	return result.Size, err
}

func (self *Fs) Users() (fsdata.IdentityList, error) {
	target, err := self.agentTarget()
	if err != nil {
		return fsdata.IdentityList{}, err
	}
	value, err := self.agent.Fs.Users(self.sdk.Ctx, target)
	if err != nil {
		return fsdata.IdentityList{}, err
	}
	result := fsdata.IdentityList{
		User:  make([]fsdata.UserIdentity, 0, len(value.User)),
		Group: make([]fsdata.GroupIdentity, 0, len(value.Group)),
	}
	for _, item := range value.User {
		result.User = append(result.User, fsdata.UserIdentity{
			Name: item.Name, UID: item.UID, GID: item.GID, Description: item.Description,
		})
	}
	for _, item := range value.Group {
		result.Group = append(result.Group, fsdata.GroupIdentity{Name: item.Name, GID: item.GID})
	}
	return result, nil
}

func (self *Fs) Copy(sourceName, targetName string, overwrite bool) error {
	source, target, agentTarget, err := self.transferTarget(sourceName, targetName)
	if err != nil {
		return err
	}
	return self.agent.Fs.Copy(self.sdk.Ctx, agentTarget, source, target, overwrite)
}

func (self *Fs) Move(sourceName, targetName string, overwrite bool) error {
	source, target, agentTarget, err := self.transferTarget(sourceName, targetName)
	if err != nil {
		return err
	}
	return self.agent.Fs.Move(self.sdk.Ctx, agentTarget, source, target, overwrite)
}

func (self *Fs) transferTarget(sourceName, targetName string) (string, string, serviceAgent.FsTarget, error) {
	source, err := self.pathName(sourceName)
	if err != nil {
		return "", "", serviceAgent.FsTarget{}, err
	}
	target, err := self.pathName(targetName)
	if err != nil {
		return "", "", serviceAgent.FsTarget{}, err
	}
	agentTarget, err := self.agentTarget()
	if err != nil {
		return "", "", serviceAgent.FsTarget{}, err
	}
	return source, target, agentTarget, nil
}

func (self *Fs) readFile(name string, target io.Writer) (err error) {
	archive, _, err := self.sdk.Client.CopyFromContainer(self.sdk.Ctx, self.targetContainerName, name)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, archive.Close())
	}()
	tarReader := tar.NewReader(archive)
	header, err := tarReader.Next()
	if err != nil {
		return err
	}
	if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
		return fmt.Errorf("unsupported archive entry type %d", header.Typeflag)
	}
	_, err = io.CopyN(target, tarReader, header.Size)
	return err
}

func (self *Fs) writeFile(backendName string, file *os.File, info os.FileInfo) (err error) {
	offset, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}
	defer func() {
		_, seekErr := file.Seek(offset, io.SeekStart)
		err = errors.Join(err, seekErr)
	}()
	fileStat, err := file.Stat()
	if err != nil {
		return err
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	archiveReader, archiveWriter := io.Pipe()
	writeResult := make(chan error, 1)
	go func() {
		tarWriter := tar.NewWriter(archiveWriter)
		header := &tar.Header{
			Name: path.Base(backendName), Mode: int64(formatTarMode(info.Mode())), Size: fileStat.Size(),
			ModTime: fileStat.ModTime(), AccessTime: fileStat.ModTime(), Format: tar.FormatPAX,
		}
		writeErr := tarWriter.WriteHeader(header)
		if writeErr == nil {
			_, writeErr = io.CopyN(tarWriter, file, fileStat.Size())
		}
		writeErr = errors.Join(writeErr, tarWriter.Close())
		_ = archiveWriter.CloseWithError(writeErr)
		writeResult <- writeErr
	}()
	copyErr := self.sdk.Client.CopyToContainer(
		self.sdk.Ctx, self.targetContainerName, path.Dir(backendName), archiveReader,
		container.CopyToContainerOptions{},
	)
	_ = archiveReader.CloseWithError(copyErr)
	return errors.Join(copyErr, <-writeResult)
}

func (self *Fs) resolvePath(name string) (string, container.PathStat, bool, error) {
	name, err := self.backendPath(name)
	if err != nil {
		return "", container.PathStat{}, false, err
	}
	remaining := strings.TrimPrefix(name, self.rootPath)
	remaining = strings.TrimPrefix(remaining, "/")
	components := splitPath(remaining)
	current := self.rootPath
	initialExists := len(components) == 0
	symlinkDepth := 0

	for {
		if len(components) > 0 {
			current = path.Join(current, components[0])
			components = components[1:]
		}
		pathStat, err := self.sdk.Client.ContainerStatPath(self.sdk.Ctx, self.targetContainerName, current)
		if err != nil {
			if len(components) > 0 {
				current = path.Join(current, path.Join(components...))
			}
			return current, container.PathStat{}, initialExists, err
		}
		if len(components) == 0 {
			initialExists = true
		}
		if pathStat.Mode&os.ModeSymlink == 0 && pathStat.LinkTarget == "" {
			if len(components) == 0 {
				return current, pathStat, initialExists, nil
			}
			continue
		}
		if pathStat.LinkTarget == "" {
			return current, container.PathStat{}, initialExists, &os.PathError{Op: "stat", Path: current, Err: errors.New("symbolic link target is empty")}
		}
		symlinkDepth++
		if symlinkDepth > maxSymlinkDepth {
			return current, container.PathStat{}, initialExists, &os.PathError{Op: "stat", Path: name, Err: errors.New("too many symbolic links")}
		}
		target := pathStat.LinkTarget
		if !path.IsAbs(target) {
			target = path.Join(path.Dir(current), target)
		}
		target = path.Clean(target)
		if len(components) > 0 {
			target = path.Join(target, path.Join(components...))
		}
		if _, err = self.virtualPath(target); err != nil {
			return target, container.PathStat{}, initialExists, &os.PathError{Op: "stat", Path: target, Err: err}
		}
		remaining = strings.TrimPrefix(target, self.rootPath)
		remaining = strings.TrimPrefix(remaining, "/")
		components = splitPath(remaining)
		current = self.rootPath
	}
}

func splitPath(name string) []string {
	if name == "" {
		return nil
	}
	return strings.Split(name, "/")
}

func (self *Fs) pathName(name string) (string, error) {
	if name == "" {
		name = self.workingDir
	}
	if strings.IndexByte(name, 0) >= 0 || !path.IsAbs(name) || path.Clean(name) != name {
		return "", errors.New("illegal path")
	}
	return name, nil
}

func (self *Fs) backendPath(name string) (string, error) {
	name, err := self.pathName(name)
	if err != nil {
		return "", err
	}
	if self.rootPath == "/" {
		return name, nil
	}
	if name == "/" {
		return self.rootPath, nil
	}
	return path.Join(self.rootPath, strings.TrimPrefix(name, "/")), nil
}

func (self *Fs) virtualPath(name string) (string, error) {
	if name == "" || strings.IndexByte(name, 0) >= 0 || !path.IsAbs(name) || path.Clean(name) != name {
		return "", errors.New("illegal backend path")
	}
	if self.rootPath == "/" {
		return name, nil
	}
	if name != self.rootPath && !strings.HasPrefix(name, self.rootPath+"/") {
		return "", errors.New("path is outside the backend root")
	}
	name = strings.TrimPrefix(name, self.rootPath)
	if name == "" {
		return "/", nil
	}
	return name, nil
}

func (self *Fs) ContainerPath(name string) (string, error) {
	name, _, _, err := self.resolvePath(name)
	if err != nil {
		return "", err
	}
	if _, err = self.virtualPath(name); err != nil {
		return "", err
	}
	return name, nil
}

func (self *Fs) fileInfo(name string, pathStat container.PathStat) os.FileInfo {
	return fsdata.NewFileInfo(&fsdata.FileData{
		Path: name, Name: path.Base(name), Mod: pathStat.Mode, ModStr: pathStat.Mode.String(), ModTime: pathStat.Mtime,
		Change: fsdata.ChangeDefault, Size: pathStat.Size, LinkName: pathStat.LinkTarget,
		IsDir: pathStat.Mode.IsDir(), IsSymlink: pathStat.Mode&os.ModeSymlink != 0,
	})
}

func (self *Fs) agentTarget() (serviceAgent.FsTarget, error) {
	if self.targetType == targetTypeMount {
		return serviceAgent.FsTarget{Root: self.rootPath}, nil
	}
	info, err := self.sdk.Client.ContainerInspect(self.sdk.Ctx, self.targetContainerName)
	if err != nil {
		return serviceAgent.FsTarget{}, err
	}
	if info.State == nil || info.State.Pid <= 1 {
		return serviceAgent.FsTarget{}, fmt.Errorf("the %s container does not exist or is not running", self.targetContainerName)
	}
	return serviceAgent.FsTarget{ContainerPID: info.State.Pid}, nil
}

func formatTarMode(mode os.FileMode) uint32 {
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
	return value
}

func isNotExist(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errdefs.IsNotFound(err)
}

func (self *Fs) Import(fileList []serviceafs.TransferFile) (err error) {
	if len(fileList) == 0 {
		return nil
	}
	optionList := make([]archiveservice.CreateOption, 0, len(fileList))
	for _, item := range fileList {
		if err = validateLocalTransferPath(item.Source, true); err != nil {
			return fmt.Errorf("invalid import source %q: %w", item.Source, err)
		}
		if _, err = self.pathName(item.Target); err != nil {
			return fmt.Errorf("invalid import target %q: %w", item.Target, err)
		}
		if err = self.validateImportTree(item.Source, item.Target); err != nil {
			return fmt.Errorf("invalid import target %q: %w", item.Target, err)
		}

		backendTarget, err := self.backendPath(item.Target)
		if err != nil {
			return err
		}
		archivePath := strings.TrimPrefix(backendTarget, "/")
		if archivePath != "" {
			optionList = append(optionList, archiveservice.WithFile(item.Source, archivePath))
			continue
		}
		info, err := os.Lstat(item.Source)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return errors.New("cannot import a regular file to the filesystem root")
		}
		entries, err := os.ReadDir(item.Source)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			optionList = append(optionList, archiveservice.WithFile(
				filepath.Join(item.Source, entry.Name()), entry.Name(),
			))
		}
	}

	temporaryFileSystem, err := tempfs.New()
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, temporaryFileSystem.Close())
	}()
	archivePath, err := temporaryFileSystem.LocalPath("/import.tar")
	if err != nil {
		return err
	}
	if err = archiveservice.CreateTar(archivePath, optionList...); err != nil {
		return err
	}
	archiveFile, err := temporaryFileSystem.Open("/import.tar")
	if err != nil {
		return err
	}
	copyErr := self.sdk.ContainerImport(self.sdk.Ctx, self.targetContainerName, "/", archiveFile)
	_ = archiveFile.Close()
	return copyErr
}

func (self *Fs) Export(fileList []serviceafs.TransferFile) error {
	for _, item := range fileList {
		if _, err := self.pathName(item.Source); err != nil {
			return fmt.Errorf("invalid export source %q: %w", item.Source, err)
		}
		if err := self.validateExportSource(item.Source); err != nil {
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

func (self *Fs) validateImportTree(source, target string) error {
	if err := self.validateImportTarget(target); err != nil {
		return err
	}
	info, err := os.Lstat(source)
	if err != nil || !info.IsDir() {
		return err
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err = self.validateImportTree(filepath.Join(source, entry.Name()), path.Join(target, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func (self *Fs) validateImportTarget(name string) error {
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
		info, _, err := self.LstatIfPossible(current)
		if isNotExist(err) {
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
	}
	return nil
}

func (self *Fs) validateExportSource(name string) error {
	info, _, err := self.LstatIfPossible(name)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() && !info.Mode().IsRegular() {
		return errors.New("source is not a regular file or directory")
	}
	if !info.IsDir() {
		return nil
	}
	entries, err := self.ReadDir(name)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err = self.validateExportSource(path.Join(name, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func (self *Fs) exportEntry(source, target string) (err error) {
	temporaryFileSystem, err := tempfs.New()
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, temporaryFileSystem.Close())
	}()

	backendSource, err := self.backendPath(source)
	if err != nil {
		return err
	}
	archiveReader, _, err := self.sdk.Client.CopyFromContainer(self.sdk.Ctx, self.targetContainerName, backendSource)
	if err != nil {
		return err
	}
	archivePath, err := temporaryFileSystem.LocalPath("/export.tar")
	if err != nil {
		_ = archiveReader.Close()
		return err
	}
	archiveFile, err := temporaryFileSystem.Create("/export.tar")
	if err != nil {
		_ = archiveReader.Close()
		return err
	}
	_, copyErr := io.Copy(archiveFile, archiveReader)
	err = errors.Join(copyErr, archiveFile.Close(), archiveReader.Close())
	if err != nil {
		return err
	}
	extractDirectory, err := temporaryFileSystem.LocalPath("/files")
	if err != nil {
		return err
	}
	if err = archiveservice.UnArchive(archivePath, extractDirectory); err != nil {
		return err
	}

	extractedSource := extractDirectory
	if source != "/" || self.rootPath != "/" {
		extractedSource = filepath.Join(extractDirectory, filepath.FromSlash(path.Base(backendSource)))
		if _, statErr := os.Lstat(extractedSource); statErr != nil {
			entries, readErr := temporaryFileSystem.ReadDir("/files")
			if readErr != nil {
				return readErr
			}
			if len(entries) != 1 {
				return errors.New("docker export archive has an unexpected layout")
			}
			extractedSource = filepath.Join(extractDirectory, entries[0].Name())
		}
	}
	return copyLocalEntry(extractedSource, target)
}

func copyLocalEntry(source, target string) error {
	info, err := os.Lstat(source)
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
		entries, err := os.ReadDir(source)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err = copyLocalEntry(filepath.Join(source, entry.Name()), filepath.Join(target, entry.Name())); err != nil {
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
	sourceFile, err := os.Open(source)
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
	if mustExist && info.IsDir() {
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
	return nil
}
