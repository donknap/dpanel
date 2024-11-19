package logic

import (
	"archive/zip"
	"errors"
	"fmt"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/exec"
	"gopkg.in/yaml.v3"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Store struct {
}

func (self Store) SyncByGit(path, gitUrl string) error {
	gitPath, _ := os.MkdirTemp("", "dpanel-store")
	defer func() {
		_ = os.RemoveAll(gitPath)
	}()
	out, err := exec.Command{}.Run(&exec.RunCommandOption{
		CmdName: "git",
		CmdArgs: []string{
			"clone", "--depth", "1",
			gitUrl, gitPath,
		},
		Timeout: time.Second * 30,
	})
	if err != nil {
		return err
	}
	_, err = io.Copy(os.Stdout, out)
	if err != nil {
		return err
	}
	savePath := filepath.Join(path, "app.zip")
	_ = os.MkdirAll(filepath.Dir(savePath), os.ModePerm)
	out, err = exec.Command{}.Run(&exec.RunCommandOption{
		CmdName: "git",
		CmdArgs: []string{
			"archive", "--format", "zip", "-o", filepath.Join(path, "app.zip"), "HEAD",
		},
		Timeout: time.Second * 30,
		Dir:     gitPath,
	})
	return nil
}

func (self Store) SyncByZip(path, zipUrl string) error {
	response, err := http.Get(zipUrl)
	if err != nil {
		return err
	}
	savePath := filepath.Join(path, "app.zip")
	_ = os.MkdirAll(filepath.Dir(savePath), os.ModePerm)
	zipFile, _ := os.OpenFile(savePath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0666)
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode != http.StatusOK {
		return errors.New("下载 zip 失败" + response.Status)
	}
	_, err = io.Copy(zipFile, response.Body)
	if err != nil {
		return err
	}
	return nil
}

func (self Store) SyncByJson(path, jsonUrl string) error {
	file, err := os.OpenFile(filepath.Join(path, "template.json"), os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0666)
	defer func() {
		_ = file.Close()
	}()

	response, err := http.Get(jsonUrl)
	if err != nil {
		return err
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusOK {
		return errors.New("下载 json 失败" + response.Status)
	}
	_, err = io.Copy(file, response.Body)
	if err != nil {
		return err
	}
	return nil
}

func (self Store) GetAppByOnePanel(appZipPath string) ([]accessor.StoreAppItem, error) {
	result := make([]accessor.StoreAppItem, 0)
	zipReader, err := zip.OpenReader(appZipPath)
	if err != nil {
		return nil, err
	}
	item := accessor.StoreAppItem{
		Version: make(map[string]accessor.StoreAppVersionItem),
	}

	for _, file := range zipReader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if !strings.HasPrefix(file.Name, "apps") {
			continue
		}
		fileReader, err := file.Open()
		if err != nil {
			return nil, err
		}
		segments := strings.Split(filepath.Clean(file.Name), string(filepath.Separator))

		if item.Name == "" {
			item.Name = segments[1]
		}

		if segments[1] != item.Name {
			result = append(result, item)
			item = accessor.StoreAppItem{
				Name:    segments[1],
				Version: make(map[string]accessor.StoreAppVersionItem),
			}
		}

		if strings.HasSuffix(file.Name, "data.yml") {
			content, err := io.ReadAll(fileReader)
			if err != nil {
				return nil, err
			}
			yamlData := new(function.YamlGetter)
			err = yaml.Unmarshal(content, &yamlData)
			if err != nil {
				return nil, err
			}
			// 应用介绍信息 data.yaml
			if len(segments) == 3 {
				item.Name = segments[1]
				item.Description = yamlData.GetString("description")
				item.Tag = yamlData.GetStringSlice("tags")
			}
			// 安装配置信息
			if len(segments) == 4 {
				versionItem := accessor.StoreAppVersionItem{
					Environment: make([]accessor.EnvItem, 0),
				}
				if _, ok := item.Version[segments[2]]; ok {
					versionItem = item.Version[segments[2]]
				}
				fmt.Printf("%v \n", file.Name)
				fields := yamlData.GetSliceStringMapString("additionalProperties.formFields")
				for _, field := range fields {
					versionItem.Environment = append(versionItem.Environment, accessor.EnvItem{
						Name:  field["envKey"],
						Value: field["default"],
						Label: field["labelZh"],
					})
				}
				item.Version[segments[2]] = versionItem
			}
		}
		if strings.HasSuffix(file.Name, "docker-compose.yml") {
			versionItem := accessor.StoreAppVersionItem{}
			if _, ok := item.Version[segments[2]]; ok {
				versionItem = item.Version[segments[2]]
			}
			versionItem.File = file.Name
			versionItem.Name = segments[2]
			item.Version[segments[2]] = versionItem
		}
		if strings.HasSuffix(file.Name, "logo.png") {
			//content, err := io.ReadAll(fileReader)
			//if err != nil {
			//	return nil, err
			//}
			//item.Logo = fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(content))
		}
	}
	return result, nil
}
