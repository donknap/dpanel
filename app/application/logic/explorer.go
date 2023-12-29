package logic

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/plugin"
	"log/slog"
	"strings"
)

func NewExplorer(md5 string) (*explorer, error) {
	containerInfo, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, md5)
	if err != nil {
		return nil, err
	}
	if containerInfo.State.Pid == 0 {
		return nil, err
	}
	explorerPlugin, err := plugin.NewPlugin("explorer")
	if err != nil {
		return nil, err
	}
	proxyName, err := explorerPlugin.Create()
	if err != nil {
		return nil, err
	}
	commander, err := plugin.Command{}.Attach(proxyName, nil)
	if err != nil {
		return nil, err
	}
	o := &explorer{
		commander: commander,
		rootPath:  fmt.Sprintf("/proc/%d/root", containerInfo.State.Pid),
	}
	return o, nil
}

type explorer struct {
	commander *plugin.Hijacked
	rootPath  string
}

func (self explorer) GetListByPath(path string) (fileList []*fileItem, err error) {
	path = strings.TrimSuffix(path, "/") + "/"
	listCmd := fmt.Sprintf("cd %s && ls -AlhX --full-time %s%s \n", self.rootPath, self.rootPath, path)
	slog.Debug("explorer", "list", listCmd)
	out, err := self.commander.Run(listCmd)
	slog.Debug("explorer", "list", string(out))
	if err != nil {
		return fileList, err
	}
	lines := bytes.Split(out, []byte("\n"))
	for _, line := range lines {
		if function.IsEmptyArray(line) {
			continue
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

func (self explorer) Unzip(rootPath string, zipName string) error {
	rootPath = strings.TrimPrefix(rootPath, "/") + "/"
	listCmd := fmt.Sprintf("cd %s/%s && unzip -o ./%s \n", self.rootPath, rootPath, zipName)
	out, err := self.commander.Run(listCmd)
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
		deleteFileList = append(deleteFileList, self.rootPath+"/"+strings.TrimPrefix(path, "/"))
	}
	listCmd := fmt.Sprintf("cd %s && rm -rf %s && pwd \n", self.rootPath, strings.Join(deleteFileList, " "))
	out, err := self.commander.Run(listCmd)
	if err != nil {
		return err
	}
	fmt.Printf("%v \n", string(out))
	return nil
}

func (self explorer) Create(path string, isDir bool) error {
	var cmd string
	if isDir {
		cmd = fmt.Sprintf("cd %s && mkdir -p %s/%s && pwd \n", self.rootPath, self.rootPath, strings.TrimPrefix(path, "/"))
	} else {
		cmd = fmt.Sprintf("cd %s && touch %s/%s && pwd \n", self.rootPath, self.rootPath, strings.TrimPrefix(path, "/"))
	}
	out, err := self.commander.Run(cmd)
	if err != nil {
		return err
	}
	fmt.Printf("%v \n", string(out))
	return nil
}
