package logic

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/errdefs"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/agent/factor"
	archiveservice "github.com/donknap/dpanel/common/service/archive"
	"github.com/donknap/dpanel/common/service/docker"
	dockerTypes "github.com/donknap/dpanel/common/service/docker/types"
	servicefs "github.com/donknap/dpanel/common/service/fs"
	serviceafs "github.com/donknap/dpanel/common/service/fs/afs"
	"github.com/donknap/dpanel/common/service/fs/dockerfs"
	"github.com/donknap/dpanel/common/service/fs/hostfs"
	"github.com/donknap/dpanel/common/service/fs/tempfs"
	"github.com/donknap/dpanel/common/service/notice"
	serviceSsh "github.com/donknap/dpanel/common/service/ssh"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/h2non/filetype"
)

const (
	ExplorerMountTypeContainer = "container"
	ExplorerMountTypeVolume    = "volume"
	ExplorerMountTypeDocker    = "docker"
	ExplorerMountTypeLocal     = "local"
	ExplorerMountHost          = "host"
	ExplorerMountDPanel        = "dpanel"
)

type Explorer struct{}

type ExplorerMountPoint string

func (self ExplorerMountPoint) mountName() string {
	_, name, _ := strings.Cut(string(self), ":")
	return name
}

func (self ExplorerMountPoint) isHostPath() bool {
	return self == ExplorerMountPoint(fmt.Sprintf("%s:%s", ExplorerMountTypeLocal, ExplorerMountHost))
}

func (self ExplorerMountPoint) isDPanelPath() bool {
	return self == ExplorerMountPoint(fmt.Sprintf("%s:%s", ExplorerMountTypeLocal, ExplorerMountDPanel))
}

func (self ExplorerMountPoint) isLocalDockerPath() bool {
	return !function.IsRunInDocker() && self == ExplorerMountPoint(fmt.Sprintf("%s:%s", ExplorerMountTypeDocker, define.DockerDefaultClientName))
}

func (self ExplorerMountPoint) isDockerPath() bool {
	return strings.HasPrefix(string(self), fmt.Sprintf("%s:", ExplorerMountTypeDocker))
}

func (self ExplorerMountPoint) isContainerPath() bool {
	return strings.HasPrefix(string(self), fmt.Sprintf("%s:", ExplorerMountTypeContainer))
}

func (self ExplorerMountPoint) isVolumePath() bool {
	return strings.HasPrefix(string(self), fmt.Sprintf("%s:", ExplorerMountTypeVolume))
}

func (self Explorer) Afs(ctx context.Context, mountPoint ExplorerMountPoint, dockerSdk *docker.Client) (serviceafs.Fs, error) {
	mountName := mountPoint.mountName()
	switch {
	case mountPoint.isHostPath(), mountPoint.isLocalDockerPath():
		// local:host 访问面板进程可见的根目录；容器运行时是面板容器的根目录。
		return newExplorerHostFs(ctx, hostfs.WithRoot("/"), hostfs.WithName(mountName))

	case mountPoint.isDPanelPath():
		// local:dpanel 在容器中访问 /dpanel，二进制运行时访问本地数据目录。
		rootPath := "/dpanel"
		if !function.IsRunInDocker() {
			rootPath = storage.Local{}.GetStorageLocalPath()
		}
		return newExplorerHostFs(ctx, hostfs.WithRoot(rootPath), hostfs.WithName(mountName))

	case mountPoint.isDockerPath():
		dockerEnv, err := (Env{}).GetEnvByName(mountName)
		if err != nil {
			return nil, err
		}
		if !dockerEnv.EnableSSH || dockerEnv.SshServerInfo == nil {
			return nil, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted)
		}
		options := []serviceSsh.Option{serviceSsh.WithContext(ctx), serviceSsh.WithSftpClient()}
		options = append(options, serviceSsh.WithServerInfo(dockerEnv.SshServerInfo)...)
		sshClient, err := serviceSsh.NewClient(options...)
		if err != nil {
			return nil, err
		}
		fileSystem, err := newExplorerHostFs(ctx,
			hostfs.WithSftpClient(sshClient.SftpConn), hostfs.WithName(mountName),
		)
		if err != nil {
			sshClient.Close()
			return nil, err
		}
		context.AfterFunc(ctx, sshClient.Close)
		return fileSystem, nil

	case mountPoint.isContainerPath():
		if dockerSdk == nil || dockerSdk.Client == nil {
			return nil, errors.New("docker client is required for this explorer mount point")
		}
		if mountName == factor.ExplorerName {
			return newExplorerAgentFs(mountPoint, mountName, dockerSdk,
				factor.ExplorerCreateOption{}, dockerfs.WithRoot("/dpanel"),
			)
		}
		containerInfo, err := dockerSdk.Client.ContainerInspect(dockerSdk.Ctx, mountName)
		if err != nil {
			return nil, err
		}
		if containerInfo.State == nil || containerInfo.State.Pid <= 1 {
			return nil, fmt.Errorf("the %s container does not exist or is not running", mountName)
		}
		workingDir := "/"
		if containerInfo.Config != nil && containerInfo.Config.WorkingDir != "" {
			workingDir = containerInfo.Config.WorkingDir
		}
		return newExplorerAgentFs(mountPoint, mountName, dockerSdk,
			factor.ExplorerCreateOption{HostPID: true},
			dockerfs.WithTargetContainer(mountName), dockerfs.WithWorkingDir(workingDir),
		)

	case mountPoint.isVolumePath():
		if dockerSdk == nil || dockerSdk.Client == nil {
			return nil, errors.New("docker client is required for this explorer mount point")
		}
		volumeInfo, err := dockerSdk.Client.VolumeInspect(dockerSdk.Ctx, mountName)
		if err != nil {
			return nil, err
		}
		mountPath := path.Join("/", volumeInfo.Name)
		return newExplorerAgentFs(mountPoint, mountName, dockerSdk,
			factor.ExplorerCreateOption{
				WorkingDir: mountPath,
				Volumes:    []dockerTypes.VolumeItem{{Host: volumeInfo.Name, Dest: mountPath, Type: "volume"}},
			},
			dockerfs.WithRoot(mountPath), dockerfs.WithWorkingDir("/"),
		)

	default:
		return nil, errors.New("invalid explorer mount point")
	}
}

func newExplorerHostFs(ctx context.Context, options ...hostfs.Option) (serviceafs.Fs, error) {
	fileSystem, err := servicefs.NewFs(servicefs.WithHostDriver(options...))
	if err != nil {
		return nil, err
	}
	context.AfterFunc(ctx, func() {
		_ = fileSystem.Destroy()
	})
	return fileSystem, nil
}

func newExplorerAgentFs(mountPoint ExplorerMountPoint, mountName string, dockerSdk *docker.Client, createOption factor.ExplorerCreateOption, fsOptions ...dockerfs.Option) (serviceafs.Fs, error) {
	mounts := (Setting{}).GetDPanelInfo().DataMounts
	if len(mounts) == 0 || mounts[0].Host == "" || mounts[0].Dest != "/dpanel" {
		return nil, errors.New("dpanel data mount is unavailable")
	}
	lock := storage.NewMutex(fmt.Sprintf(storage.CacheKeyExplorerAfsLock, dockerSdk.Name, factor.ExplorerName))
	lock.Lock()
	defer lock.Unlock()

	createOption.Hash = function.Sha256Struct(struct {
		MountPoint ExplorerMountPoint
		DataMounts []dockerTypes.VolumeItem
	}{mountPoint, mounts})
	createOption.Volumes = append(append([]dockerTypes.VolumeItem(nil), mounts...), createOption.Volumes...)
	if _, err := factor.NewExplorer(dockerSdk, createOption); err != nil {
		return nil, err
	}
	fsOptions = append([]dockerfs.Option{
		dockerfs.WithName(mountName),
		dockerfs.WithDockerSdk(dockerSdk),
		dockerfs.WithProxyContainer(factor.ExplorerName),
	}, fsOptions...)
	return servicefs.NewFs(servicefs.WithDockerDriver(fsOptions...))
}

type ExplorerImportFile struct {
	Name string
	Path string
}

type ExplorerContent struct {
	Content  string
	FileMode uint32
}

type ExplorerDownload struct {
	*os.File
	path        string
	FileName    string
	ContentType string
}

func (self *ExplorerDownload) Close() error {
	return errors.Join(self.File.Close(), os.Remove(self.path))
}

func (self Explorer) Export(fileSystem serviceafs.Fs, fileList []string, exportToPanelPath bool) (*ExplorerDownload, error) {
	if len(fileList) == 0 {
		return nil, errors.New("export file list is empty")
	}
	if !exportToPanelPath && len(fileList) == 1 {
		info, _, err := fileSystem.LstatIfPossible(fileList[0])
		if err != nil {
			return nil, err
		}
		if info.Mode().IsRegular() {
			target, err := storage.Local{}.CreateTempFile("")
			if err != nil {
				return nil, err
			}
			targetPath := target.Name()
			if err = target.Close(); err != nil {
				_ = os.Remove(targetPath)
				return nil, err
			}
			if err = fileSystem.Export([]serviceafs.TransferFile{{Source: fileList[0], Target: targetPath}}); err != nil {
				_ = os.Remove(targetPath)
				return nil, err
			}
			target, err = os.Open(targetPath)
			if err != nil {
				_ = os.Remove(targetPath)
				return nil, err
			}
			return &ExplorerDownload{File: target, path: targetPath, FileName: info.Name(), ContentType: "application/octet-stream"}, nil
		}
	}
	temporaryFileSystem, err := tempfs.New()
	if err != nil {
		return nil, err
	}
	defer temporaryFileSystem.Close()

	transferList := make([]serviceafs.TransferFile, 0, len(fileList))
	optionList := make([]archiveservice.CreateOption, 0, len(fileList))
	for index, sourcePath := range fileList {
		archiveName := path.Base(sourcePath)
		localPath, pathErr := temporaryFileSystem.LocalPath(path.Join("/", strconv.Itoa(index), archiveName))
		if pathErr != nil {
			return nil, pathErr
		}
		transferList = append(transferList, serviceafs.TransferFile{Source: sourcePath, Target: localPath})
		optionList = append(optionList, archiveservice.WithFile(localPath, archiveName))
	}
	if err = fileSystem.Export(transferList); err != nil {
		return nil, err
	}

	var target *os.File
	if exportToPanelPath {
		target, err = storage.Local{}.CreateSaveFile("export/file/" + fileSystem.Name() + "-" + time.Now().Format(define.DateYmdHis) + ".zip")
	} else {
		target, err = storage.Local{}.CreateTempFile("")
	}
	if err != nil {
		return nil, err
	}
	targetPath := target.Name()
	if err = target.Close(); err != nil {
		_ = os.Remove(targetPath)
		return nil, err
	}
	if err = archiveservice.Create(targetPath, archiveservice.FormatZip, optionList...); err != nil {
		_ = os.Remove(targetPath)
		return nil, err
	}
	if exportToPanelPath {
		_ = notice.Message{}.Info(define.InfoMessageCommonExportInPath, "path", targetPath)
		return nil, nil
	}
	target, err = os.Open(targetPath)
	if err != nil {
		_ = os.Remove(targetPath)
		return nil, err
	}
	return &ExplorerDownload{
		File: target, path: targetPath,
		FileName:    "export-" + fileSystem.Name() + "-" + time.Now().Format("20060102-150405") + ".zip",
		ContentType: "application/zip",
	}, nil
}

func (self Explorer) ImportFileContent(fileSystem serviceafs.Fs, fileName, content, destination string, fileMode uint32) error {
	info, err := fileSystem.Stat(destination)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("destination is not a directory")
	}
	targetPath := path.Join(destination, fileName)
	mode := os.FileMode(fileMode)
	if targetInfo, statErr := fileSystem.Stat(targetPath); statErr == nil {
		if targetInfo.IsDir() {
			return errors.New("target is a directory")
		}
		if mode == 0 {
			mode = targetInfo.Mode().Perm()
		}
	} else if !errors.Is(statErr, os.ErrNotExist) && !errdefs.IsNotFound(statErr) {
		return statErr
	}
	if mode == 0 {
		mode = 0o666
	}
	file, err := fileSystem.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, writeErr := io.WriteString(file, content)
	return errors.Join(writeErr, file.Close())
}

func (self Explorer) Import(fileSystem serviceafs.Fs, destination string, fileList []ExplorerImportFile) (err error) {
	files := make([]serviceafs.TransferFile, 0, len(fileList))
	for _, item := range fileList {
		realPath := function.SafePathJoin(storage.Local{}.GetLocalTempDir(), function.SlashPathToSystem(item.Path))
		files = append(files, serviceafs.TransferFile{Source: realPath, Target: path.Join(destination, item.Name)})
	}
	if err = fileSystem.Import(files); err != nil {
		return err
	}
	for _, item := range files {
		err = errors.Join(err, os.Remove(item.Source))
	}
	return err
}

func (self Explorer) UnArchive(fileSystem serviceafs.Fs, archives []string, destination string) error {
	if len(archives) == 0 {
		return errors.New("archive file list is empty")
	}
	destinationInfo, err := fileSystem.Stat(destination)
	if err != nil {
		return err
	}
	if !destinationInfo.IsDir() {
		return errors.New("archive destination is not a directory")
	}
	err = fileSystem.UnArchive(archives, destination)
	if errors.Is(err, archiveservice.ErrUnsupportedFormat) {
		return function.ErrorMessage(define.ErrorMessageContainerExplorerUnzipTargetUnsupportedType)
	}
	return err
}

func (self Explorer) GetContent(fileSystem serviceafs.Fs, filePath string) (ExplorerContent, error) {
	const maxContentSize = 1024 * 1024
	fileInfo, err := fileSystem.Stat(filePath)
	if err != nil {
		return ExplorerContent{}, err
	}
	if !fileInfo.Mode().IsRegular() {
		return ExplorerContent{}, function.ErrorMessage(define.ErrorMessageContainerExplorerContentUnsupportedType)
	}
	file, err := fileSystem.Open(filePath)
	if err != nil {
		return ExplorerContent{}, err
	}
	defer file.Close()
	fileInfo, err = file.Stat()
	if err != nil {
		return ExplorerContent{}, err
	}
	if !fileInfo.Mode().IsRegular() {
		return ExplorerContent{}, function.ErrorMessage(define.ErrorMessageContainerExplorerContentUnsupportedType)
	}
	if fileInfo.Size() >= maxContentSize {
		return ExplorerContent{}, function.ErrorMessage(define.ErrorMessageContainerExplorerEditFileMaxSize)
	}
	content, err := io.ReadAll(io.LimitReader(file, maxContentSize))
	if err != nil {
		return ExplorerContent{}, err
	}
	if len(content) >= maxContentSize {
		return ExplorerContent{}, function.ErrorMessage(define.ErrorMessageContainerExplorerEditFileMaxSize)
	}
	fileType, _ := filetype.Match(content)
	if fileType != filetype.Unknown {
		return ExplorerContent{}, function.ErrorMessage(define.ErrorMessageContainerExplorerContentUnsupportedType)
	}
	return ExplorerContent{Content: string(content), FileMode: uint32(fileInfo.Mode().Perm())}, nil
}

func (self Explorer) Permission(fileSystem serviceafs.Fs, fileList []string, mode string, uid, gid *int, recursive bool) error {
	if mode == "" && uid == nil && gid == nil {
		return errors.New("mod, uid, or gid is required")
	}
	if uid != nil && *uid < 0 || gid != nil && *gid < 0 {
		return errors.New("uid and gid cannot be negative")
	}
	var fileMode *os.FileMode
	if mode != "" {
		value, err := strconv.ParseUint(mode, 8, 12)
		if err != nil || value > 0o7777 {
			return fmt.Errorf("invalid mode %q", mode)
		}
		parsed := os.FileMode(value & 0o777)
		if value&0o4000 != 0 {
			parsed |= os.ModeSetuid
		}
		if value&0o2000 != 0 {
			parsed |= os.ModeSetgid
		}
		if value&0o1000 != 0 {
			parsed |= os.ModeSticky
		}
		fileMode = &parsed
	}
	var err error
	for _, filePath := range fileList {
		if fileMode != nil {
			err = fileSystem.ChmodAll(filePath, *fileMode, recursive)
		}
		if err == nil && (uid != nil || gid != nil) {
			err = fileSystem.ChownAll(filePath, uid, gid, recursive)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (self Explorer) Archive(fileSystem serviceafs.Fs, sources []string, target string, format archiveservice.Format, overwrite bool) error {
	if len(sources) == 0 {
		return errors.New("archive file list is empty")
	}
	if targetInfo, err := fileSystem.Stat(target); err == nil {
		if !overwrite {
			return os.ErrExist
		}
		if !targetInfo.Mode().IsRegular() {
			return errors.New("archive target is not a regular file")
		}
	} else if !errors.Is(err, os.ErrNotExist) && !errdefs.IsNotFound(err) {
		return err
	}
	temporaryFileSystem, err := tempfs.New()
	if err != nil {
		return err
	}
	defer temporaryFileSystem.Close()
	transferList := make([]serviceafs.TransferFile, 0, len(sources))
	optionList := make([]archiveservice.CreateOption, 0, len(sources))
	for index, source := range sources {
		archiveName := path.Base(source)
		localPath, pathErr := temporaryFileSystem.LocalPath(path.Join("/", strconv.Itoa(index), archiveName))
		if pathErr != nil {
			return pathErr
		}
		transferList = append(transferList, serviceafs.TransferFile{Source: source, Target: localPath})
		optionList = append(optionList, archiveservice.WithFile(localPath, archiveName))
	}
	if err = fileSystem.Export(transferList); err != nil {
		return err
	}
	localTarget, err := temporaryFileSystem.LocalPath("/archive")
	if err != nil {
		return err
	}
	if err = archiveservice.Create(localTarget, format, optionList...); err != nil {
		return err
	}
	return fileSystem.Import([]serviceafs.TransferFile{{Source: localTarget, Target: target}})
}
