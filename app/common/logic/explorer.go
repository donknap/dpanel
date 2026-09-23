package logic

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/docker/docker/errdefs"
	"github.com/donknap/dpanel/common/function"
	serviceagent "github.com/donknap/dpanel/common/service/agent"
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
		mounts := (Setting{}).GetDPanelInfo().DataMounts
		if len(mounts) == 0 || mounts[0].Host == "" || mounts[0].Dest != "/dpanel" {
			return nil, errors.New("dpanel data mount is unavailable")
		}
		mountPointValue := mountType + ":" + mountName + ":" + function.Sha256Struct(mounts)
		lock := storage.NewMutex(fmt.Sprintf(storage.CacheKeyExplorerAfsLock, dockerSdk.Name, plugin.ExplorerName))
		lock.Lock()
		defer lock.Unlock()

		pluginOption := plugin.CreateOption{Init: true, Hash: mountPointValue, Volumes: mounts}
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

		return servicefs.NewFs(servicefs.WithDockerDriver(dockerFsOptions...))
	default:
		return nil, errors.New("unknown explorer mount point type")
	}
}

func (self Explorer) DestroyProxyContainer(dockerSdk *docker.Client) error {
	if dockerSdk == nil || dockerSdk.Client == nil {
		return errors.New("docker client is required to destroy explorer proxy")
	}
	lock := storage.NewMutex(fmt.Sprintf(storage.CacheKeyExplorerAfsLock, dockerSdk.Name, plugin.ExplorerName))
	lock.Lock()
	defer lock.Unlock()
	explorerPlugin, err := plugin.NewPlugin(dockerSdk, plugin.ExplorerName, plugin.CreateOption{Init: false})
	if err != nil {
		return err
	}
	return explorerPlugin.Close()
}

func (self Explorer) SyncDPanelDirectory(ctx context.Context, dockerSdk *docker.Client, sourcePath, targetPath string) error {
	temporary, err := storage.Local{}.CreateTempFile("")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	if err = temporary.Close(); err != nil {
		_ = os.Remove(temporaryPath)
		return err
	}
	defer os.Remove(temporaryPath)
	if err = archiveservice.CreateTar(temporaryPath, archiveservice.WithFile(sourcePath, targetPath)); err != nil {
		return fmt.Errorf("archive dpanel directory: %w", err)
	}
	archive, err := os.Open(temporaryPath)
	if err != nil {
		return err
	}
	defer archive.Close()
	if _, err = self.Afs(ctx, ExplorerMountTypeContainer, plugin.ExplorerName, dockerSdk); err != nil {
		return fmt.Errorf("prepare explorer for dpanel sync: %w", err)
	}
	agent, err := serviceagent.NewDockerAgent(dockerSdk, plugin.ExplorerName)
	if err != nil {
		return err
	}
	if err = agent.ImportDPanel(ctx, archive); !errdefs.IsNotFound(err) {
		return err
	}
	if _, retryErr := self.Afs(ctx, ExplorerMountTypeContainer, plugin.ExplorerName, dockerSdk); retryErr != nil {
		return errors.Join(err, fmt.Errorf("recreate explorer for dpanel sync: %w", retryErr))
	}
	if _, retryErr := archive.Seek(0, io.SeekStart); retryErr != nil {
		return errors.Join(err, fmt.Errorf("rewind dpanel archive: %w", retryErr))
	}
	return agent.ImportDPanel(ctx, archive)
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
