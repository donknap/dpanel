package dockerfs

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/docker/go-units"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/donknap/dpanel/common/types/fs"
	"github.com/spf13/afero"
	"github.com/spf13/afero/mem"
)

type Fs struct {
	sdk                     *docker.Client
	targetContainerName     string // 目标容器名称
	targetContainerRootPath string // 目标容器的起始根目录
	proxyContainerName      string
}

func New(opts ...Option) (afero.Fs, error) {
	var err error

	o := &Fs{}
	for _, opt := range opts {
		err = opt(o)
		if err != nil {
			return nil, err
		}
	}

	if o.sdk == nil {
		return nil, errors.New("invalid docker sdk")
	}

	if o.targetContainerName == "" || o.targetContainerRootPath == "" {
		return nil, fmt.Errorf("the %s container does not exist or is not running", o.targetContainerName)
	}

	return o, nil
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

func (self Fs) MkdirAll(p string, perm os.FileMode) error {
	p, err := self.getSafePath(p)
	if err != nil {
		return err
	}
	cmd := fmt.Sprintf("mkdir -p \"%s\"", p)
	_, err = self.sdk.ContainerExecResult(self.sdk.Ctx, self.proxyContainerName, cmd)
	if err != nil {
		return err
	}
	return nil
}

func (self Fs) Open(name string) (afero.File, error) {
	statPath, err := self.sdk.Client.ContainerStatPath(self.sdk.Ctx, self.targetContainerName, name)
	if err != nil {
		return nil, err
	}
	fileData := &fs.FileData{
		Path:     filepath.Join(self.targetContainerRootPath, name),
		Name:     name,
		Mod:      statPath.Mode,
		ModTime:  statPath.Mtime,
		Change:   fs.ChangeDefault,
		Size:     statPath.Size,
		User:     "",
		Group:    "",
		LinkName: statPath.LinkTarget,
	}
	if statPath.Mode.IsDir() {
		return &File{
			fd:   nil,
			fs:   &self,
			info: fs.NewFileInfo(fileData),
		}, nil
	}
	file := mem.NewFileHandle(mem.CreateFile(name))
	out, err := self.sdk.ContainerReadFile(self.sdk.Ctx, self.targetContainerName, name, nil)
	if err != nil {
		return nil, err
	}
	_, _ = io.Copy(file, out)
	_, _ = file.Seek(0, io.SeekStart)
	return &File{
		fd:   file,
		fs:   &self,
		info: fs.NewFileInfo(fileData),
	}, nil
}

func (self Fs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	//TODO implement me
	panic("implement me")
}

func (self Fs) Remove(name string) error {
	//TODO implement me
	panic("implement me")
}

func (self Fs) RemoveAll(p string) error {
	p, err := self.getSafePath(p)
	if err != nil {
		return err
	}
	cmd := fmt.Sprintf("rm -rf \"%s\"", p)
	_, err = self.sdk.ContainerExecResult(self.sdk.Ctx, self.proxyContainerName, cmd)
	if err != nil {
		return err
	}
	return nil
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
	p, err := self.getSafePath(name)
	if err != nil {
		return err
	}
	cmd := fmt.Sprintf("chmod -R %d %s", mode, p)
	_, err = self.sdk.ContainerExecResult(self.sdk.Ctx, self.proxyContainerName, cmd)
	if err != nil {
		return err
	}
	return nil
}

func (self Fs) Chown(name string, uid, gid int) error {
	//TODO implement me
	panic("implement me")
}

func (self Fs) Chtimes(name string, atime time.Time, mtime time.Time) error {
	//TODO implement me
	panic("implement me")
}

func (self Fs) readDirFromContainer(rootPath string) ([]os.FileInfo, error) {
	rootPath, err := self.getSafePath(rootPath)
	if err != nil {
		return nil, err
	}
	cmd := fmt.Sprintf("ls -AlhX --full-time %s", rootPath)
	cmd += " | awk 'NR>1 {printf \"{d>%s<d}\", $1;for (i=2; i<=NF; i++) printf \"{v>%s<v}\", $i;}'"
	out, err := self.sdk.ContainerExecResult(self.sdk.Ctx, self.proxyContainerName, cmd)
	if err != nil {
		return nil, err
	}
	//lines := strings.Split(out, "\t")
	// 这里不能单纯的用换行进行分隔，正常的数据中会有多余的 \n
	lines := make([][]byte, 0)
	reg := regexp.MustCompile(`\{d>[a-zA-Z-][a-zA-Z-]{3}[a-zA-Z-]{3}[a-zA-Z-]{3}<d\}`).FindAllStringIndex(out, -1)
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
			return nil, function.ErrorMessage(define.ErrorMessageUnknow, "error", string(line))
		}

		modStr := strings.Trim(row[0], "`")
		size, _ := units.FromHumanSize(row[4])
		modTime, _ := time.Parse(define.DateShowYmdHis, row[5]+" "+row[6])

		if !function.IsEmptyArray(row) {
			fileData := &fs.FileData{
				Path:     "",
				Name:     strings.Join(row[8:], " "),
				Size:     size,
				Mod:      self.mod(modStr),
				ModStr:   modStr,
				Change:   fs.ChangeDefault,
				ModTime:  modTime,
				User:     row[2],
				Group:    row[3],
				LinkName: "",
			}
			if len(row) >= 10 && row[0][0] == 'l' {
				// 如果当前是软链接，还需要再次检查目录是目录还是文件
				if name, linkName, ok := strings.Cut(fileData.Name, "->"); ok {
					fileData.LinkName = strings.TrimSpace(linkName)
					fileData.Name = strings.TrimSpace(name)
				}
			}
			rel, _ := filepath.Rel(self.targetContainerRootPath, rootPath)
			// 因为路径使终是从 Linux 容器或是系统中获取，不能使用 filepath，使用 path 保持路径 / 分隔
			fileData.Path = path.Join("/", rel, fileData.Name)
			fileData.IsDir = fileData.Mod.IsDir()
			fileData.IsSymlink = fileData.CheckIsSymlink()
			fileList = append(fileList, fs.NewFileInfo(fileData))
		}
	}
	if function.IsEmptyArray(fileList) {
		return fileList, nil
	}
	return fileList, nil
}

func (self Fs) getSafePath(rootPath string) (string, error) {
	if rootPath != function.PathClean(rootPath) {
		return "", errors.New("illegal path")
	}
	rootPath = function.PathClean(rootPath)
	// 目录必须是以 / 开头
	if !strings.HasPrefix(rootPath, "/") {
		return "", errors.New("please use absolute address")
	}
	if rootPath == "/" {
		// 根目录必须加上 / 表示是目录
		return fmt.Sprintf("%s/", self.targetContainerRootPath), nil
	}
	return path.Join(self.targetContainerRootPath, rootPath), nil
}

func (self Fs) mod(s string) os.FileMode {
	var mode os.FileMode

	permStart := len(s) - 9
	if permStart < 0 {
		panic("invalid file mode string")
	}

	specialPart := s[:permStart]
	permPart := s[permStart:]

	// 1. 处理特殊标志部分（如目录、符号链接等）
	for _, c := range specialPart {
		switch c {
		case 'd': // 目录
			mode |= os.ModeDir
		case 'l': // 符号链接
			mode |= os.ModeSymlink
		case 'c': // 字符设备
			mode |= os.ModeCharDevice | os.ModeDevice
		case 'b': // 块设备
			mode |= os.ModeDevice
		case 's': // Socket
			mode |= os.ModeSocket
		case 'p': // 管道
			mode |= os.ModeNamedPipe
		default:
			// 其他类型或忽略未知字符
		}
	}

	// 2. 处理权限部分（rwxrwxrwx）
	for j, c := range permPart {
		if c != '-' {
			bitPos := 8 - j
			mode |= 1 << uint(bitPos)
		}
	}

	return mode
}
