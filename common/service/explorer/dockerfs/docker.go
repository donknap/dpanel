package dockerfs

import (
	"errors"
	"fmt"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/explorer"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/spf13/afero"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const tempTarName = "dpanel-plugin-explorer.tar"

type Fs struct {
	sdk                     *docker.Builder
	rootPath                string // 在本地对应的临时目录
	proxyContainerName      string
	targetContainerName     string
	targetContainerRootPath string
}

func New(sdk *docker.Builder, targetContainerName string, proxyContainerName string) (afero.Fs, error) {
	if proxyContainerName == "" {
		proxyContainerName = targetContainerName
	}
	containerInfo, err := sdk.Client.ContainerInspect(sdk.Ctx, targetContainerName)
	if err != nil {
		return nil, err
	}
	if containerInfo.State.Pid == 0 {
		return nil, fmt.Errorf("the %s container does not exist or is not running", targetContainerName)
	}
	path, err := storage.Local{}.CreateTempDir("container-rootfs-" + targetContainerName)
	if err != nil {
		return nil, err
	}
	return &Fs{
		sdk:                     sdk,
		proxyContainerName:      proxyContainerName,
		targetContainerName:     targetContainerName,
		targetContainerRootPath: fmt.Sprintf("/proc/%d/root", containerInfo.State.Pid),
		rootPath:                filepath.Base(path),
	}, nil
}

func (self Fs) Name() string {
	return "docker fs"
}

func (self Fs) Create(name string) (afero.File, error) {
	//TODO implement me
	panic("implement me")
}

func (self Fs) Mkdir(name string, perm os.FileMode) error {
	//TODO implement me
	panic("implement me")
}

func (self Fs) MkdirAll(path string, perm os.FileMode) error {
	//TODO implement me
	panic("implement me")
}

func (self Fs) Open(name string) (afero.File, error) {
	statPath, err := self.sdk.Client.ContainerStatPath(self.sdk.Ctx, self.targetContainerName, name)
	if err != nil {
		return nil, err
	}
	fileData := &explorer.FileData{
		Name:     name,
		Mod:      statPath.Mode,
		ModTime:  statPath.Mtime,
		Change:   explorer.ChangeDefault,
		Size:     statPath.Size,
		Owner:    "",
		Group:    "",
		LinkName: statPath.LinkTarget,
	}
	if statPath.Mode.IsDir() {
		return &File{
			fd:   nil,
			fs:   &self,
			info: explorer.NewFileInfo(fileData),
		}, nil
	}
	tempFile, err := storage.Local{}.CreateTempFile("")
	_, err = self.sdk.ContainerReadFile(self.sdk.Ctx, self.proxyContainerName, "/"+tempTarName, tempFile)
	if err != nil {
		return nil, err
	}
	return &File{
		fd:   tempFile,
		fs:   &self,
		info: explorer.NewFileInfo(fileData),
	}, nil
}

func (self Fs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	var tempFile *os.File
	var err error

	if file, err := self.Open(name); err == nil {
		if v, ok := file.(*File); ok {
			if v.info.Mode().IsDir() {
				return nil, &os.PathError{
					Op:   "OpenFile",
					Path: name,
					Err:  errors.New("target is a directory"),
				}
			}
			tempFile = v.fd
		}
	}

	if tempFile == nil {
		tempFile, err = storage.Local{}.CreateTempFile(filepath.Join(self.rootPath, name))
		if err != nil {
			return nil, err
		}
		defer func() {
			_ = tempFile.Close()
		}()
	}

	file, err := os.OpenFile(tempFile.Name(), flag, perm)
	if err != nil {
		return nil, err
	}
	fileInfo, _ := file.Stat()
	return &File{
		fd:   file,
		fs:   &self,
		info: fileInfo,
	}, nil
}

func (self Fs) Remove(name string) error {
	//TODO implement me
	panic("implement me")
}

func (self Fs) RemoveAll(path string) error {
	//TODO implement me
	panic("implement me")
}

func (self Fs) Rename(oldname, newname string) error {
	//TODO implement me
	panic("implement me")
}

func (self Fs) Stat(name string) (os.FileInfo, error) {
	//TODO implement me
	panic("implement me")
}

func (self Fs) Chmod(name string, mode os.FileMode) error {
	//TODO implement me
	panic("implement me")
}

func (self Fs) Chown(name string, uid, gid int) error {
	//TODO implement me
	panic("implement me")
}

func (self Fs) Chtimes(name string, atime time.Time, mtime time.Time) error {
	//TODO implement me
	panic("implement me")
}

func (self Fs) commit(containerPathName string, sourceFile *os.File) error {
	// close 的时候删除临时文件，并提交到容器内
	importFile, err := docker.NewFileImport("/", docker.WithImportFilePath(sourceFile.Name(), containerPathName))
	defer func() {
		_ = sourceFile.Close()
		_ = os.Remove(sourceFile.Name())
	}()
	if err != nil {
		return err
	}
	err = self.sdk.ContainerImport(self.sdk.Ctx, self.targetContainerName, importFile)
	if err != nil {
		return err
	}
	return nil
}

func (self Fs) readDirFromContainer(path string) ([]os.FileInfo, error) {
	path, err := self.getSafePath(path)
	if err != nil {
		return nil, err
	}
	cmd := fmt.Sprintf("ls -AlhX --full-time %s", path)
	cmd += " | awk 'NR>1 {printf \"{d>%s<d}\", $1;for (i=2; i<=NF; i++) printf \"{v>%s<v}\", $i;}'"
	out, err := self.sdk.ContainerExecResult(self.sdk.Ctx, self.proxyContainerName, cmd)
	if err != nil {
		return nil, err
	}
	//lines := strings.Split(out, "\t")
	// 这里不能单纯的用换行进行分隔，正常的数据中会有多余的 \n
	lines := make([][]byte, 0)
	reg := regexp.MustCompile(`\{d>[a-zA-Z-][a-zA-Z-]{3}[a-zA-Z-]{3}[a-zA-Z-]{3}<d\}`).FindAllStringIndex(string(out), -1)
	for i, _ := range reg {
		line := make([]byte, 0)
		start, end := reg[i][0], 0
		if i+1 >= len(reg) {
			end = len(out)
		} else {
			end = reg[i+1][0]
		}
		line = append(line, out[start:end]...)
		lines = append(lines, line)
	}
	fileList := make([]os.FileInfo, 0)
	for i, line := range lines {
		// 只提取 {v>%s<v} 定位符之间的内容
		row := make([]string, 0)
		reg := regexp.MustCompile(`\{[v|d]>(.*?)<[v|d]\}`).FindAllIndex(line, -1)
		for _, pos := range reg {
			row = append(row, string(line[pos[0]+3:pos[1]-3]))
		}
		if len(row) < 8 {
			slog.Debug("explorer", "get-path-list", i, "line", string(line))
			return nil, errors.New("目录解析错误, 请反馈: " + string(line))
		}
		row[0] = strings.Trim(row[0], "`")
		switch row[0][0] {
		case 'd', 'l', '-', 'b':
			size, _ := strconv.ParseInt(row[4], 10, 64)
			modTime, _ := time.Parse(function.ShowYmdHis, row[5]+" "+row[6])
			var mode fs.FileMode
			switch row[0][0] {
			case 'd':
				mode = os.ModeDir
			case 'l':
				mode = os.ModeSymlink
			case 'c':
				mode = os.ModeCharDevice
			case 'b':
				mode = os.ModeDevice
			case 'p':
				mode = os.ModeNamedPipe
			case 'S':
				mode = os.ModeSocket
			case '-':
				mode = 0
			default:
				mode = 0
			}
			if !function.IsEmptyArray(row) {
				fileData := &explorer.FileData{
					Name:     strings.Join(row[8:], " "),
					Size:     size,
					Mod:      mode,
					Change:   explorer.ChangeDefault,
					ModTime:  modTime,
					Owner:    row[2],
					Group:    row[3],
					LinkName: "",
				}
				if len(row) >= 10 && row[0][0] == 'l' {
					// 如果当前是软链接，还需要再次检查目录是目录还是文件
					if name, linkName, ok := strings.Cut(fileData.Name, "->"); ok {
						fileData.LinkName = strings.TrimSpace(linkName)
						fileData.Name = name
					}
				}
				fileData.IsDir = fileData.Mod.IsDir()
				fileData.IsSymlink = fileData.CheckIsSymlink()
				fileList = append(fileList, explorer.NewFileInfo(fileData))
			}
		}
	}
	if function.IsEmptyArray(fileList) {
		return fileList, nil
	}
	return fileList, nil
}

func (self Fs) getSafePath(path string) (string, error) {
	// 目录必须是以 / 开头
	if !strings.HasPrefix(path, "/") {
		return "", errors.New("please use absolute address")
	}
	if path == "/" {
		// 根目录必须加上 / 表示是目录
		return fmt.Sprintf("%s/", self.targetContainerRootPath), nil
	}
	return filepath.Join(self.targetContainerRootPath, path), nil
}
