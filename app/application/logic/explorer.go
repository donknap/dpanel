package logic

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/plugin"
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
	explorerPlugin, err := plugin.NewPlugin("explorer", nil)
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

func (self explorer) GetListByPath(path string) (fileList []*fileItemResult, err error) {
	path, err = self.getSafePath(path)
	if err != nil {
		return fileList, err
	}
	cmd := fmt.Sprintf("ls -AlhX --full-time %s%s \n", self.rootPath, path)
	slog.Debug("explorer", "get-list", cmd)
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
				item := &fileItemResult{
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
	cmd := fmt.Sprintf("cd %s && rm -rf \"%s\" \n", self.rootPath, strings.Join(deleteFileList, " "))
	out, err := plugin.Command{}.Result(self.pluginName, cmd)
	if err != nil {
		return err
	}
	slog.Debug("explorer", "delete", string(out))
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
	slog.Debug("explorer", "create", string(out))
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
	slog.Debug("explorer", "chmod", string(out))
	return nil
}

// Chown 更改文件所属用户时，由于变更的用户在 explorer 中可能不存在，只能在当前容器中操作
func (self explorer) Chown(containerName string, fileList []string, owner string, hasChildren bool) error {
	var changeFileList []string
	for _, path := range fileList {
		if !strings.HasPrefix(path, "/") {
			return errors.New("please use absolute address")
		}
		changeFileList = append(changeFileList, path)
	}
	flag := ""
	if hasChildren {
		flag += " -R "
	}
	cmd := fmt.Sprintf("chown %s %s:%s %s \n", flag, owner, owner, strings.Join(changeFileList, " "))
	out, err := plugin.Command{}.Result(containerName, cmd)
	if err != nil {
		return err
	}
	slog.Debug("explorer", "chown", string(out))
	return nil
}

func (self explorer) GetPasswd() ([]*userItemResult, error) {
	result := make([]*userItemResult, 0)
	cmd := fmt.Sprintf("cd %s && cat etc/passwd \n", self.rootPath)
	out, err := plugin.Command{}.Result(self.pluginName, cmd)
	if err != nil {
		return result, err
	}
	slog.Debug("explorer", "passwd", string(out))

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if len(line) > 8 {
			switch stdcopy.StdType(line[0]) {
			case stdcopy.Stdin, stdcopy.Stdout, stdcopy.Stderr, stdcopy.Systemerr:
				line = line[8:]
			}
		}
		detail := strings.Split(string(line), ":")
		if len(line) < 7 {
			continue
		}
		result = append(result, &userItemResult{
			Username:    detail[0],
			UID:         detail[2],
			GID:         detail[3],
			Description: detail[4],
			HomePath:    detail[5],
		})
	}
	return result, nil
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
	slog.Debug("explorer", "rename", string(out))
	return nil
}
