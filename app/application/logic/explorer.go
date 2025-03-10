package logic

import (
	"errors"
	"fmt"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/plugin"
	"log/slog"
	"regexp"
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
	explorerPlugin, err := plugin.NewPlugin(plugin.PluginExplorer, nil)
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
	cmd = fmt.Sprintf("ls -AlhX --full-time %s%s", self.rootPath, path)
	cmd += " | awk 'NR>1 {printf \"{d>%s<d}\", $1;for (i=2; i<=NF; i++) printf \"{v>%s<v}\", $i;}'"
	out, err := plugin.Command{}.Result(self.pluginName, cmd)
	if err != nil {
		return fileList, err
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
			if !function.IsEmptyArray(row) {
				item := &fileItemResult{
					ShowName: strings.Join(row[8:], " "),
					IsDir:    row[0][0] == 'd',
					Size:     row[4],
					Mode:     row[0],
					Change:   -1,
					ModTime:  row[5] + " " + row[6],
					Owner:    row[2],
					Group:    row[3],
				}
				if len(row) >= 10 && row[0][0] == 'l' {
					item.LinkName = " "
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
	cmd := fmt.Sprintf("cd %s && rm -rf \"%s\" \n", self.rootPath, strings.Join(deleteFileList, "\" \""))
	_, err := plugin.Command{}.Result(self.pluginName, cmd)
	if err != nil {
		return err
	}
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
	_, err := plugin.Command{}.Result(self.pluginName, cmd)
	if err != nil {
		return err
	}
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
	_, err := plugin.Command{}.Result(containerName, cmd)
	if err != nil {
		return err
	}
	return nil
}

func (self explorer) getSafePath(path string) (string, error) {
	if !strings.HasPrefix(path, "/") {
		return "", errors.New("please use absolute address")
	}
	return strings.TrimSuffix(path, "/") + "/", nil
}
