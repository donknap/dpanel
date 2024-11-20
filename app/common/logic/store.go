package logic

import (
	"archive/zip"
	"errors"
	"fmt"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/exec"
	"github.com/donknap/dpanel/common/service/storage"
	"gopkg.in/yaml.v3"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Store struct {
}

func (self Store) SyncByGit(path, gitUrl string) error {
	out, err := exec.Command{}.Run(&exec.RunCommandOption{
		CmdName: "git",
		CmdArgs: []string{
			"clone", "--depth", "1",
			gitUrl, path,
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
	return nil
}

func (self Store) SyncByZip(path, zipUrl string) error {
	zipTempFile, _ := os.CreateTemp("", "dpanel-store")
	defer func() {
		_ = zipTempFile.Close()
		_ = os.RemoveAll(zipTempFile.Name())
	}()

	response, err := http.Get(zipUrl)
	if err != nil {
		return err
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode != http.StatusOK {
		return errors.New("下载 zip 失败" + response.Status)
	}
	_, err = io.Copy(zipTempFile, response.Body)
	if err != nil {
		return err
	}
	zipArchive, err := zip.OpenReader(zipTempFile.Name())
	if err != nil {
		return err
	}
	defer func() {
		_ = zipArchive.Close()
	}()
	for _, file := range zipArchive.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if strings.HasPrefix(file.Name, "__MACOSX") {
			continue
		}
		targetFilePath := filepath.Join(path, file.Name)
		err = os.MkdirAll(filepath.Dir(targetFilePath), os.ModePerm)
		if err != nil {
			return err
		}
		targetFile, err := os.OpenFile(targetFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}
		sourceFile, _ := file.Open()
		_, _ = io.Copy(targetFile, sourceFile)
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

func (self Store) GetAppByOnePanel(storePath string) ([]accessor.StoreAppItem, error) {
	if !filepath.IsAbs(storePath) {
		storePath = filepath.Join(storage.Local{}.GetStorePath(), storePath, "apps")
	}
	fmt.Printf("%v \n", storePath)
	result := make([]accessor.StoreAppItem, 0)
	item := accessor.StoreAppItem{
		Version: make(map[string]accessor.StoreAppVersionItem),
	}

	err := filepath.Walk(storePath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relPath, _ := filepath.Rel(storePath, path)
		segments := strings.Split(filepath.Clean(relPath), string(filepath.Separator))

		if item.Name == "" {
			item.Name = segments[0]
		}

		if segments[0] != item.Name {
			result = append(result, item)
			item = accessor.StoreAppItem{
				Name:    segments[0],
				Version: make(map[string]accessor.StoreAppVersionItem),
				Logo:    fmt.Sprintf("%s/logo.png", segments[0]),
			}
		}

		if strings.HasSuffix(relPath, "data.yml") {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			yamlData := new(function.YamlGetter)
			err = yaml.Unmarshal(content, &yamlData)
			if err != nil {
				return err
			}

			// 应用介绍信息 data.yaml
			if len(segments) == 2 {
				item.Name = segments[0]
				item.Description = yamlData.GetString("description")
				item.Tag = yamlData.GetStringSlice("tags")
			}

			// 安装配置信息
			if len(segments) == 3 {
				versionItem := accessor.StoreAppVersionItem{
					Environment: make([]accessor.EnvItem, 0),
				}
				if _, ok := item.Version[segments[1]]; ok {
					versionItem = item.Version[segments[1]]
				}
				fields := yamlData.GetSliceStringMapString("additionalProperties.formFields")
				for _, field := range fields {
					versionItem.Environment = append(versionItem.Environment, accessor.EnvItem{
						Name:  field["envKey"],
						Value: field["default"],
						Label: field["labelZh"],
					})
				}
				item.Version[segments[1]] = versionItem
			}
		}
		if strings.HasSuffix(relPath, "docker-compose.yml") {
			versionItem := accessor.StoreAppVersionItem{}
			if _, ok := item.Version[segments[1]]; ok {
				versionItem = item.Version[segments[1]]
			}
			versionItem.File = relPath
			versionItem.Name = segments[1]
			item.Version[segments[1]] = versionItem
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
