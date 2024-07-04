package logic

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/plugin"
	"github.com/h2non/filetype"
	"log/slog"
	"path/filepath"
	"strings"
)

func NewExplorer(md5 string) (*explorer, error) {
	containerInfo, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, md5)
	if err != nil {
		return nil, err
	}
	if containerInfo.State.Pid == 0 {
		return nil, errors.New("please start the container" + md5)
	}
	explorerPlugin, err := plugin.NewPlugin("explorer")
	if err != nil {
		return nil, err
	}
	pluginName, err := explorerPlugin.Create()
	if err != nil {
		return nil, err
	}
	o := &explorer{
		rootPath:   fmt.Sprintf("/proc/%d/root", containerInfo.State.Pid),
		pluginName: pluginName,
	}
	return o, nil
}

type explorer struct {
	pluginName string
	rootPath   string
}

func (self explorer) GetListByPath(path string) (fileList []*fileItem, err error) {
	path, err = self.getSafePath(path)
	if err != nil {
		return fileList, err
	}
	cmd := fmt.Sprintf("ls -AlhX --full-time %s%s \n", self.rootPath, path)
	slog.Debug("explorer", "cmd", cmd)
	out, err := plugin.Command{}.Result(self.pluginName, cmd)
	if err != nil {
		return fileList, err
	}
	lines := bytes.Split(out, []byte("\n"))
	for _, line := range lines {
		if function.IsEmptyArray(line) {
			continue
		}
		if len(line) > 8 {
			switch stdcopy.StdType(line[0]) {
			case stdcopy.Stdin, stdcopy.Stdout, stdcopy.Stderr, stdcopy.Systemerr:
				line = line[8:]
			}
		}
		switch line[0] {
		case 'd', 'l', '-', 'b':
			row := strings.Fields(string(line))
			if !function.IsEmptyArray(row) {
				item := &fileItem{
					ShowName: string(line[strings.LastIndex(string(line), row[8]):]),
					IsDir:    line[0] == 'd',
					Size:     row[4],
					Mode:     row[0],
					Change:   -1,
					ModTime:  row[5] + row[6],
					Owner:    row[2],
					Group:    row[3],
				}
				if strings.Contains(item.ShowName, "->") {
					index := strings.Index(item.ShowName, "->")
					item.LinkName = item.ShowName[index+2:]
					item.ShowName = item.ShowName[0:index]
				}
				item.Name = path + item.ShowName
				fileList = append(fileList, item)
			}
		}
	}
	if function.IsEmptyArray(fileList) {
		return fileList, nil
	}
	return fileList, nil
}

func (self explorer) Unzip(path string, zipName string) error {
	path, err := self.getSafePath(path)
	if err != nil {
		return err
	}
	cmd := fmt.Sprintf("cd %s/%s && unzip -o ./%s \n", self.rootPath, path, zipName)
	out, err := plugin.Command{}.Result(self.pluginName, cmd)

	if err != nil {
		return err
	}
	if !strings.Contains(string(out), "inflating") {
		return errors.New(string(out))
	}
	return nil
}

func (self explorer) DeleteFileList(fileList []string) error {
	var deleteFileList []string
	for _, path := range fileList {
		if !strings.HasPrefix(path, "/") {
			return errors.New("please use absolute address")
		}
		deleteFileList = append(deleteFileList, self.rootPath+path)
	}
	cmd := fmt.Sprintf("cd %s && rm -rf %s \n", self.rootPath, strings.Join(deleteFileList, " "))
	out, err := plugin.Command{}.Result(self.pluginName, cmd)
	if err != nil {
		return err
	}
	slog.Debug("explorer", "out", string(out))
	return nil
}

// Deprecated: 获取文件不采用 shell 命令，不稳定且需要借助 file 命令才能判断文件类型
// file 在 busybox 和 alpine 并未支持
// 获取主谁的内容先把文件下载本地生成临时文件，再通过文件的导入提交修改
func (self explorer) GetContent(file string) (string, error) {
	if !strings.HasPrefix(file, "/") {
		return "", errors.New("please use absolute address")
	}
	file = fmt.Sprintf("%s%s", self.rootPath, file)
	cmd := fmt.Sprintf(`cat %s \n`, file)
	out, err := plugin.Command{}.Result(self.pluginName, cmd)
	if err != nil {
		return "", err
	}
	if len(out) <= 8 {
		return "", nil
	}
	return string(out[8:]), nil
}

func (self explorer) GetContentByTar(reader *tar.Reader) (string, error) {
	file, err := reader.Next()
	if err != nil {
		return "", nil
	}
	if file.Typeflag != tar.TypeReg {
		return "", errors.New("不支持编辑的文件类型")
	}
	if file.Size >= 1024*1024 {
		return "", errors.New("超过1M的文件请通过导入&导出修改文件")
	}
	content := make([]byte, file.Size)
	reader.Read(content)

	fileType, _ := filetype.Match(content)
	if fileType == filetype.Unknown {
		return string(content), nil
	}
	return "", nil
}

func (self explorer) getSafePath(path string) (string, error) {
	if !strings.HasPrefix(path, "/") {
		return "", errors.New("please use absolute address")
	}
	return strings.TrimSuffix(path, "/") + "/", nil
}

// Deprecated: 无用
func (self explorer) Create(path string, isDir bool) error {
	path, err := self.getSafePath(path)
	if err != nil {
		return err
	}
	var cmd string
	currentPath := fmt.Sprintf("%s%s", self.rootPath, path)
	if isDir {
		cmd = fmt.Sprintf(
			`mkdir -p %s/NewFolder$(ls -al %s | grep NewFolder | wc -l | awk '{sub(/^[ \t]+/, ""); print $1+1}') \n`,
			currentPath,
			currentPath)
	} else {
		cmd = fmt.Sprintf(
			`touch %s/NewFile$(ls -al %s | grep NewFile | wc -l | awk '{sub(/^[ \t]+/, ""); print $1+1}') \n`,
			currentPath,
			currentPath)
	}
	out, err := plugin.Command{}.Result(self.pluginName, cmd)
	if err != nil {
		return err
	}
	slog.Debug("explorer", "out", string(out))
	return nil
}

func (self explorer) Chmod(fileList []string, mod int, hasChildren bool) error {
	var changeFileList []string
	for _, path := range fileList {
		if !strings.HasPrefix(path, "/") {
			return errors.New("please use absolute address")
		}
		changeFileList = append(changeFileList, self.rootPath+path)
	}
	flag := ""
	if hasChildren {
		flag += " -R "
	}
	cmd := fmt.Sprintf("cd %s && chmod %s %d %s \n", self.rootPath, flag, mod, strings.Join(changeFileList, " "))
	out, err := plugin.Command{}.Result(self.pluginName, cmd)
	if err != nil {
		return err
	}
	slog.Debug("explorer", "out", string(out))
	return nil
}

func (self explorer) Chown(fileList []string, owner string, hasChildren bool) error {
	var changeFileList []string
	for _, path := range fileList {
		if !strings.HasPrefix(path, "/") {
			return errors.New("please use absolute address")
		}
		changeFileList = append(changeFileList, self.rootPath+path)
	}
	flag := ""
	if hasChildren {
		flag += " -R "
	}
	cmd := fmt.Sprintf("cd %s && chown %s %s %s \n", self.rootPath, flag, owner, strings.Join(changeFileList, " "))
	out, err := plugin.Command{}.Result(self.pluginName, cmd)
	if err != nil {
		return err
	}
	slog.Debug("explorer", "out", string(out))
	return nil
}

// Deprecated: 无用
func (self explorer) Rename(file string, newFileName string) error {
	if !strings.HasPrefix(file, "/") || strings.Contains(newFileName, "/") {
		return errors.New("please use absolute address")
	}
	oldFile := fmt.Sprintf("%s%s", self.rootPath, file)
	newFile := fmt.Sprintf("%s/%s", filepath.Dir(oldFile), newFileName)
	cmd := fmt.Sprintf("mv %s %s \n", oldFile, newFile)
	out, err := plugin.Command{}.Result(self.pluginName, cmd)
	if err != nil {
		return err
	}
	slog.Debug("explorer", "out", string(out))
	return nil
}
