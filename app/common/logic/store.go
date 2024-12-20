package logic

import (
	"archive/zip"
	"errors"
	"fmt"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/compose"
	"github.com/donknap/dpanel/common/service/exec"
	"github.com/donknap/dpanel/common/service/storage"
	"gopkg.in/yaml.v3"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	exec2 "os/exec"
	"path/filepath"
	"strings"
	"time"
)

type StoreLogoFileSystem struct {
	fs.FS
}

func (self StoreLogoFileSystem) Open(name string) (fs.File, error) {
	return os.Open(filepath.Join(storage.Local{}.GetStorePath(), name))
}

type Store struct {
}

func (self Store) SyncByGit(path, gitUrl string) error {
	if _, err := exec2.LookPath("git"); err != nil {
		return errors.New("同步商店仓库需要使用 git 命令，请先安装")
	}
	// 先创建一个临时目录，下载完成后再同步数据，否则失败时原先的数据会被删除
	tempDownloadPath, _ := os.MkdirTemp("", "dpanel-store")
	defer func() {
		_ = os.RemoveAll(tempDownloadPath)
	}()
	slog.Debug("store git download", "path", tempDownloadPath)

	out, err := exec.Command{}.Run(&exec.RunCommandOption{
		CmdName: "git",
		CmdArgs: []string{
			"clone", "--depth", "1",
			gitUrl, tempDownloadPath,
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
	err = os.RemoveAll(path)
	if err != nil {
		return err
	}
	err = os.CopyFS(path, os.DirFS(tempDownloadPath))
	if err != nil {
		return err
	}
	return nil
}

func (self Store) SyncByZip(path, zipUrl string, root string) error {
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
		if root != "" {
			if before, after, exists := strings.Cut(file.Name, root); exists {
				if before != "" {
					targetFilePath = filepath.Join(path, root, after)
				}
			} else {
				continue
			}
		}
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

// 1panel 需要创建 1panel-network 网络
func (self Store) GetAppByOnePanel(storePath string) ([]accessor.StoreAppItem, error) {
	if !filepath.IsAbs(storePath) {
		storePath = filepath.Join(storage.Local{}.GetStorePath(), storePath, "apps")
	}
	result := make([]accessor.StoreAppItem, 0)

	storeItem := accessor.StoreAppItem{
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

		if storeItem.Name == "" {
			storeItem.Name = segments[0]
		}

		if segments[0] != storeItem.Name {
			result = append(result, storeItem)

			storeItem = accessor.StoreAppItem{
				Name:    segments[0],
				Version: make(map[string]accessor.StoreAppVersionItem),
			}
		}

		storeVersionItem := accessor.StoreAppVersionItem{
			Script:      &accessor.StoreAppVersionScriptItem{},
			Environment: make([]accessor.EnvItem, 0),
		}

		if len(segments) >= 2 {
			if _, ok := storeItem.Version[segments[1]]; ok {
				storeVersionItem = storeItem.Version[segments[1]]
			}
			defer func() {
				if storeVersionItem.Name != "" ||
					len(storeVersionItem.Environment) > 0 ||
					storeVersionItem.Script.Install != "" ||
					storeVersionItem.Script.Upgrade != "" ||
					storeVersionItem.Script.Uninstall != "" {
					storeItem.Version[segments[1]] = storeVersionItem
				}
			}()
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
				storeItem.Description = yamlData.GetString("additionalProperties.shortDescZh")
				storeItem.Tag = yamlData.GetStringSlice("additionalProperties.tags")
				storeItem.Website = yamlData.GetString("additionalProperties.website")
				storeItem.Title = yamlData.GetString("additionalProperties.name")
			}

			// 版本配置信息一个 data.yaml 为一个版本
			if len(segments) == 3 {
				fields := yamlData.GetSliceStringMapString("additionalProperties.formFields")
				env := make([]accessor.EnvItem, 0)
				env = append(env, accessor.EnvItem{
					Name:  "CONTAINER_NAME",
					Label: "容器名称",
					Value: compose.ContainerDefaultName,
				})
				for _, field := range fields {
					env = append(env, accessor.EnvItem{
						Name:  field["envKey"],
						Value: field["default"],
						Label: field["labelZh"],
					})
				}
				storeVersionItem.Environment = env
			}
		}

		if strings.HasSuffix(relPath, "/scripts/install.sh") {
			storeVersionItem.Script.Install = relPath
		}
		if strings.HasSuffix(relPath, "/scripts/uninstall.sh") {
			storeVersionItem.Script.Uninstall = relPath
		}
		if strings.HasSuffix(relPath, "/scripts/upgrade.sh") {
			storeVersionItem.Script.Upgrade = relPath
		}

		if strings.HasSuffix(relPath, "docker-compose.yml") {
			versionPath, _ := filepath.Rel(filepath.Dir(filepath.Dir(storePath)), path)
			storeVersionItem.ComposeFile = versionPath
			storeVersionItem.Name = segments[1]
		}

		r := time.Now().Unix()
		if strings.HasSuffix(relPath, "logo.png") {
			logoPath, _ := filepath.Rel(filepath.Dir(filepath.Dir(storePath)), path)
			storeItem.Logo = fmt.Sprintf("image://%s?r=%d", logoPath, r)
		}

		if strings.HasSuffix(relPath, "README.md") {
			readmePath, _ := filepath.Rel(filepath.Dir(filepath.Dir(storePath)), path)
			storeItem.Content = fmt.Sprintf("markdown-file://%s?r=%d", readmePath, r)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (self Store) GetAppByCasaos(storePath string) ([]accessor.StoreAppItem, error) {
	if !filepath.IsAbs(storePath) {
		storePath = filepath.Join(storage.Local{}.GetStorePath(), storePath, "Apps")
	}
	result := make([]accessor.StoreAppItem, 0)

	storeItem := accessor.StoreAppItem{
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

		if storeItem.Name == "" {
			storeItem.Name = segments[0]
		}

		if segments[0] != storeItem.Name {
			result = append(result, storeItem)

			storeItem = accessor.StoreAppItem{
				Name:    segments[0],
				Version: make(map[string]accessor.StoreAppVersionItem),
			}
		}
		r := time.Now().Unix()
		if strings.HasSuffix(relPath, "docker-compose.yml") {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			yamlData := new(function.YamlGetter)
			err = yaml.Unmarshal(content, &yamlData)
			if err != nil {
				return err
			}
			storeItem.Description = yamlData.GetString("x-casaos.description.zh_cn") + "\n" + yamlData.GetString("x-casaos.description.en_us")
			storeItem.Tag = []string{
				yamlData.GetString("x-casaos.category"),
			}
			storeItem.Logo = yamlData.GetString("x-casaos.icon")
			readme := yamlData.GetString("x-casaos.tips.before_install.zh_cn")
			if readme != "" {
				storeItem.Content = "markdown://" + yamlData.GetString("x-casaos.tips.before_install.zh_cn")
			}
			versionPath, _ := filepath.Rel(filepath.Dir(filepath.Dir(storePath)), path)
			storeItem.Version["latest"] = accessor.StoreAppVersionItem{
				Name:        "latest",
				ComposeFile: versionPath,
				Environment: make([]accessor.EnvItem, 0),
			}
		}

		if strings.HasSuffix(relPath, "README.md") {
			readmePath, _ := filepath.Rel(filepath.Dir(filepath.Dir(storePath)), path)
			storeItem.Content = fmt.Sprintf("markdown-file://%s?r=%d", readmePath, r)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
