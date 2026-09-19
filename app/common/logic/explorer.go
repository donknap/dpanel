package logic

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/docker/docker/errdefs"
	"github.com/donknap/dpanel/common/function"
	archiveservice "github.com/donknap/dpanel/common/service/archive"
	"github.com/donknap/dpanel/common/service/compose"
	"github.com/donknap/dpanel/common/service/docker"
	servicefs "github.com/donknap/dpanel/common/service/fs"
	serviceafs "github.com/donknap/dpanel/common/service/fs/afs"
	"github.com/donknap/dpanel/common/service/fs/dockerfs"
	"github.com/donknap/dpanel/common/service/fs/hostfs"
	"github.com/donknap/dpanel/common/service/fs/tempfs"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/plugin"
	serviceSsh "github.com/donknap/dpanel/common/service/ssh"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/h2non/filetype"
	cache "github.com/patrickmn/go-cache"
)

const (
	ExplorerMountTypeContainer = "container"
	ExplorerMountTypeVolume    = "volume"
	ExplorerMountTypeDocker    = "docker"
	ExplorerMountTypeLocal     = "local"
	ExplorerMountHost          = "host"
	ExplorerMountDPanel        = "dpanel"
)

var explorerSessionLocks sync.Map

type Explorer struct{}

type explorerSession struct {
	mountPoint string
	fileSystem serviceafs.Fs
}

func (self Explorer) Afs(ctx context.Context, mountType, mountName string, dockerSdk *docker.Client) (serviceafs.Fs, error) {
	switch mountType {
	case ExplorerMountTypeLocal:
		rootPath := "/"
		if mountName == ExplorerMountDPanel {
			rootPath = "/dpanel"
			if !function.IsRunInDocker() {
				rootPath = storage.Local{}.GetStorageLocalPath()
			}
		}
		fileSystem, err := servicefs.NewFs(servicefs.WithHostDriver(
			hostfs.WithRoot(rootPath), hostfs.WithName(mountName),
		))
		if err != nil {
			return nil, err
		}
		context.AfterFunc(ctx, func() {
			_ = fileSystem.Destroy()
		})
		return fileSystem, nil
	case ExplorerMountTypeDocker:
		if !function.IsRunInDocker() && mountName == define.DockerDefaultClientName {
			fileSystem, err := servicefs.NewFs(servicefs.WithHostDriver(
				hostfs.WithRoot("/"), hostfs.WithName(mountName),
			))
			if err != nil {
				return nil, err
			}
			context.AfterFunc(ctx, func() {
				_ = fileSystem.Destroy()
			})
			return fileSystem, nil
		}
		dockerEnv, err := (Env{}).GetEnvByName(mountName)
		if err != nil {
			return nil, err
		}
		if !dockerEnv.EnableSSH || dockerEnv.SshServerInfo == nil {
			return nil, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted)
		}
		options := []serviceSsh.Option{
			serviceSsh.WithContext(ctx),
			serviceSsh.WithSftpClient(),
		}
		options = append(options, serviceSsh.WithServerInfo(dockerEnv.SshServerInfo)...)
		sshClient, err := serviceSsh.NewClient(options...)
		if err != nil {
			return nil, err
		}
		fileSystem, err := servicefs.NewFs(servicefs.WithHostDriver(
			hostfs.WithSftpClient(sshClient.SftpConn), hostfs.WithName(mountName),
		))
		if err != nil {
			sshClient.Close()
			return nil, err
		}
		context.AfterFunc(ctx, sshClient.Close)
		return fileSystem, nil
	case ExplorerMountTypeContainer, ExplorerMountTypeVolume:
		if dockerSdk == nil || dockerSdk.Client == nil {
			return nil, errors.New("docker client is required for this explorer mount point")
		}
		mountPointValue := mountType + ":" + mountName
		key := fmt.Sprintf(storage.CacheKeyExplorerAfs, dockerSdk.Name, plugin.ExplorerName)
		lockValue, _ := explorerSessionLocks.LoadOrStore(dockerSdk.Name, &sync.Mutex{})
		lock := lockValue.(*sync.Mutex)
		lock.Lock()
		defer lock.Unlock()
		if session, ok := storage.LoadCache[*explorerSession](key); ok && session.mountPoint == mountPointValue {
			return session.fileSystem, nil
		}

		pluginOption := plugin.CreateOption{Hash: mountPointValue}
		dockerFsOptions := []dockerfs.Option{
			dockerfs.WithName(mountName),
			dockerfs.WithDockerSdk(dockerSdk),
			dockerfs.WithProxyContainer(plugin.ExplorerName),
		}
		if mountType == ExplorerMountTypeVolume {
			volumeInfo, err := dockerSdk.Client.VolumeInspect(dockerSdk.Ctx, mountName)
			if err != nil {
				return nil, err
			}
			mountPath := path.Join("/", volumeInfo.Name)
			pluginOption.WorkingDir = mountPath
			pluginOption.ExtService = compose.ExtService{External: compose.ExternalItem{Volumes: []string{
				fmt.Sprintf("%s:%s", volumeInfo.Name, mountPath),
			}}}
			dockerFsOptions = append(dockerFsOptions, dockerfs.WithRoot(mountPath), dockerfs.WithWorkingDir("/"))
		} else {
			pluginOption.HostPID = true
			dockerFsOptions = append(dockerFsOptions, dockerfs.WithTargetContainer(mountName))
		}

		explorerPlugin, err := plugin.NewPlugin(dockerSdk, plugin.ExplorerName, pluginOption)
		if err != nil {
			return nil, err
		}
		if err = explorerPlugin.Create(); err != nil {
			return nil, err
		}
		dockerFsOptions = append(dockerFsOptions, dockerfs.WithDestroy(func() error {
			lock.Lock()
			defer lock.Unlock()
			storage.Cache.Delete(key)
			return explorerPlugin.Close()
		}))
		if mountType == ExplorerMountTypeContainer {
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
			dockerFsOptions = append(dockerFsOptions, dockerfs.WithWorkingDir(workingDir))
		}

		fileSystem, err := servicefs.NewFs(servicefs.WithDockerDriver(dockerFsOptions...))
		if err != nil {
			return nil, err
		}
		storage.Cache.Set(key, &explorerSession{mountPoint: mountPointValue, fileSystem: fileSystem}, cache.NoExpiration)
		return fileSystem, nil
	default:
		return nil, errors.New("unknown explorer mount point type")
	}
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
	path string
}

func (self *ExplorerDownload) Close() error {
	return errors.Join(self.File.Close(), os.Remove(self.path))
}

func (self Explorer) Export(fileSystem serviceafs.Fs, fileList []string, exportToPanelPath bool) (*ExplorerDownload, error) {
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
	return &ExplorerDownload{File: target, path: targetPath}, nil
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
		realPath := storage.Local{}.GetSaveRealPath(function.SystemPathFromSlash(item.Path))
		files = append(files, serviceafs.TransferFile{Source: realPath, Target: path.Join(destination, item.Name)})
		defer func(filePath string) {
			err = errors.Join(err, os.Remove(filePath))
		}(realPath)
	}
	return fileSystem.Import(files)
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
	temporaryFileSystem, err := tempfs.New()
	if err != nil {
		return err
	}
	defer temporaryFileSystem.Close()
	if err = temporaryFileSystem.MkdirAll("/archives", 0o700); err != nil {
		return err
	}
	extractDirectory, err := temporaryFileSystem.LocalPath("/files")
	if err != nil {
		return err
	}
	transferList := make([]serviceafs.TransferFile, 0, len(archives))
	localArchives := make([]string, 0, len(archives))
	for index, archivePath := range archives {
		localPath, pathErr := temporaryFileSystem.LocalPath(path.Join("/archives", strconv.Itoa(index), path.Base(archivePath)))
		if pathErr != nil {
			return pathErr
		}
		transferList = append(transferList, serviceafs.TransferFile{Source: archivePath, Target: localPath})
		localArchives = append(localArchives, localPath)
	}
	if err = fileSystem.Export(transferList); err == nil {
		for _, archivePath := range localArchives {
			if err = archiveservice.UnArchive(archivePath, extractDirectory); err != nil {
				break
			}
		}
	}
	if errors.Is(err, archiveservice.ErrUnsupportedFormat) {
		return function.ErrorMessage(define.ErrorMessageContainerExplorerUnzipTargetUnsupportedType)
	}
	if err != nil {
		return err
	}
	entries, err := temporaryFileSystem.ReadDir("/files")
	if err != nil {
		return fmt.Errorf("read extracted archive files: %w", err)
	}
	transferList = make([]serviceafs.TransferFile, 0, len(entries))
	for _, entry := range entries {
		localPath, pathErr := temporaryFileSystem.LocalPath(path.Join("/files", entry.Name()))
		if pathErr != nil {
			return pathErr
		}
		transferList = append(transferList, serviceafs.TransferFile{Source: localPath, Target: path.Join(destination, entry.Name())})
	}
	return fileSystem.Import(transferList)
}

func (self Explorer) GetContent(fileSystem serviceafs.Fs, filePath string) (ExplorerContent, error) {
	file, err := fileSystem.Open(filePath)
	if err != nil {
		return ExplorerContent{}, err
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		return ExplorerContent{}, err
	}
	if fileInfo.Size() >= 1024*1024 {
		return ExplorerContent{}, function.ErrorMessage(define.ErrorMessageContainerExplorerEditFileMaxSize)
	}
	content, err := io.ReadAll(file)
	if err != nil {
		return ExplorerContent{}, err
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
