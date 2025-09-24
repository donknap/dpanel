package logic

import (
	"archive/zip"
	"fmt"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/exec/local"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
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
		return function.ErrorMessage(define.ErrorMessageSystemStoreNotFoundGit)
	}
	// 先创建一个临时目录，下载完成后再同步数据，否则失败时原先的数据会被删除
	tempDownloadPath, _ := storage.Local{}.CreateTempDir("")
	defer func() {
		_ = os.RemoveAll(tempDownloadPath)
	}()
	slog.Debug("store git download", "path", tempDownloadPath)

	cmd, err := local.New(
		local.WithCommandName("git"),
		local.WithArgs("clone", "--depth", "1",
			gitUrl, tempDownloadPath),
	)
	if err != nil {
		return err
	}
	time.AfterFunc(time.Minute*5, func() {
		_ = cmd.Close()
	})
	_, err = cmd.RunWithResult()
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
	zipTempFile, _ := storage.Local{}.CreateTempFile("")
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
		return function.ErrorMessage(define.ErrorMessageSystemStoreDownloadFailed, "url", zipUrl, "error", response.Status)
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
		return function.ErrorMessage(define.ErrorMessageSystemStoreDownloadFailed, "url", jsonUrl, "error", response.Status)
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

	err := filepath.WalkDir(storePath, func(path string, d fs.DirEntry, err error) error {
		if path == storePath {
			return nil
		}
		if !d.IsDir() {
			return nil
		}

		appName, _ := filepath.Rel(storePath, path)
		appPath := filepath.Join(storePath, appName)

		// 忽略不支持的应用
		ignoreApp := []string{
			"php5", "php7", "php8",
		}
		if function.InArray(ignoreApp, appName) {
			return filepath.SkipDir
		}

		storeItem := accessor.StoreAppItem{
			Name:     appName,
			Version:  make(map[string]accessor.StoreAppVersionItem),
			Contents: make(map[string]string),
		}

		content, err := os.ReadFile(filepath.Join(appPath, "data.yml"))
		if err != nil {
			return err
		}
		yamlData := new(function.YamlGetter)
		err = yaml.Unmarshal(content, &yamlData)
		if err != nil {
			return err
		}
		storeItem.Description = yamlData.GetString("additionalProperties.shortDescZh")
		storeItem.Descriptions = function.PluckMapWalk(yamlData.GetStringMapString("additionalProperties.description"), func(k string, v string) bool {
			return function.InArray([]string{
				"zh", "en",
			}, k)
		})
		storeItem.Tag = yamlData.GetStringSlice("additionalProperties.tags")
		storeItem.Website = yamlData.GetString("additionalProperties.website")
		storeItem.Title = yamlData.GetString("additionalProperties.name")

		resourcePath, _ := filepath.Rel(filepath.Dir(filepath.Dir(storePath)), appPath)
		r := time.Now().Unix()
		r = 1758687972
		if _, err := os.Stat(filepath.Join(appPath, "logo.png")); err == nil {
			storeItem.Logo = fmt.Sprintf("image://%s/logo.png?r=%d", resourcePath, r)
		}

		if _, err := os.Stat(filepath.Join(appPath, "README.md")); err == nil {
			storeItem.Content = fmt.Sprintf("markdown-file://%s/README.md?r=%d", resourcePath, r)
			storeItem.Contents["zh"] = fmt.Sprintf("markdown-file://%s/README.md?r=%d", resourcePath, r)
		}
		if _, err := os.Stat(filepath.Join(appPath, "README_en.md")); err == nil {
			storeItem.Contents["en"] = fmt.Sprintf("markdown-file://%s/README_en.md?r=%d", resourcePath, r)
		}

		err = filepath.WalkDir(appPath, func(path string, d fs.DirEntry, err error) error {
			if path == appPath {
				return nil
			}
			if !d.IsDir() {
				return nil
			}
			versionName, _ := filepath.Rel(appPath, path)
			versionPath := filepath.Join(appPath, versionName)

			storeVersionItem := accessor.StoreAppVersionItem{
				Script:      map[string]string{},
				Environment: make([]docker.EnvItem, 0),
				Name:        versionName,
			}

			storeVersionItem.Environment = append(storeVersionItem.Environment, self.appendOnePanelEnv()...)

			var composeYaml string
			if v, err := os.ReadFile(filepath.Join(versionPath, "docker-compose.yml")); err == nil {
				storeVersionItem.ComposeFile = filepath.Join(resourcePath, versionName, "docker-compose.yml")
				composeYaml = string(v)
			}
			for envName, envItem := range self.getOnePanelYamlEnv(storeVersionItem) {
				if strings.Contains(composeYaml, envName) {
					storeVersionItem.Environment = append(storeVersionItem.Environment, envItem)
				}
			}

			content, err := os.ReadFile(filepath.Join(versionPath, "data.yml"))
			if err != nil {
				return err
			}
			yamlData := new(function.YamlGetter)
			err = yaml.Unmarshal(content, &yamlData)
			if err != nil {
				return err
			}
			if v := self.parseOnePanelSetting(yamlData, "additionalProperties.formFields"); v != nil {
				storeVersionItem.Environment = append(storeVersionItem.Environment, v...)
			}

			for _, name := range []string{
				"install.sh", "upgrade.sh", "init.sh", "uninstall.sh",
			} {
				if _, err := os.Stat(filepath.Join(versionPath, "scripts", name)); err == nil {
					storeVersionItem.Script[name] = filepath.Join("scripts", name)
				}
			}

			if _, err := os.Stat(filepath.Join(versionPath, "build", "docker-compose.yml")); err == nil {
				task := &accessor.StoreAppVersionTaskItem{
					Name:             "build",
					Environment:      nil,
					BuildComposeFile: filepath.Join(resourcePath, versionName, "build", "docker-compose.yml"),
				}
				if v, err := os.ReadFile(filepath.Join(versionPath, "build", "config.json")); err == nil {
					jsonData := new(function.YamlGetter)
					err = yaml.Unmarshal(v, &jsonData)
					if err != nil {
						return err
					}
					task.Environment = self.parseOnePanelSetting(jsonData, "formFields")
				}
				storeVersionItem.Depend = task
			}

			storeItem.Version[versionName] = storeVersionItem
			// 找到版本目录即可
			return filepath.SkipDir
		})

		if err == nil {
			result = append(result, storeItem)
		}
		return filepath.SkipDir
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

	err := filepath.WalkDir(storePath, func(path string, d fs.DirEntry, err error) error {
		if path == storePath {
			return nil
		}
		if !d.IsDir() {
			return nil
		}

		appName, _ := filepath.Rel(storePath, path)
		appPath := filepath.Join(storePath, appName)

		storeItem := accessor.StoreAppItem{
			Name:     appName,
			Version:  make(map[string]accessor.StoreAppVersionItem),
			Contents: make(map[string]string),
		}

		composeYaml, err := os.ReadFile(filepath.Join(appPath, "docker-compose.yml"))
		if err != nil {
			return err
		}
		yamlData := new(function.YamlGetter)
		err = yaml.Unmarshal(composeYaml, &yamlData)
		if err != nil {
			return err
		}
		storeItem.Description = yamlData.GetString("x-casaos.description.zh_cn")
		storeItem.Descriptions = map[string]string{
			"zh": yamlData.GetString("x-casaos.description.zh_cn"),
			"en": yamlData.GetString("x-casaos.description.en_us"),
		}
		storeItem.Tag = []string{
			yamlData.GetString("x-casaos.category"),
		}
		storeItem.Logo = yamlData.GetString("x-casaos.icon")
		if v := yamlData.GetString("x-casaos.tips.before_install.zh_cn"); v != "" {
			storeItem.Content = "markdown-file://" + v
			storeItem.Contents["zh"] = "markdown://" + v
		}
		if v := yamlData.GetString("x-casaos.tips.before_install.en_us"); v != "" {
			storeItem.Contents["en"] = "markdown://" + v
		}
		resourcePath, _ := filepath.Rel(filepath.Dir(filepath.Dir(storePath)), appPath)
		storeItem.Version["latest"] = accessor.StoreAppVersionItem{
			Name:        "latest",
			ComposeFile: filepath.Join(resourcePath, "docker-compose.yml"),
			Environment: make([]docker.EnvItem, 0),
		}
		if err == nil {
			result = append(result, storeItem)
		}
		return filepath.SkipDir
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}

func (self Store) ParseSettingField(field map[string]string, call func(item *docker.ValueRuleItem)) *docker.ValueRuleItem {
	valueRule := &docker.ValueRuleItem{}

	if field["required"] == "true" {
		valueRule.Kind |= docker.EnvValueRuleRequired
	}
	if field["disabled"] == "true" {
		valueRule.Kind |= docker.EnvValueRuleDisabled
	}

	switch field["type"] {
	case "text":
		valueRule.Kind |= docker.EnvValueTypeText
		break
	case "number":
		valueRule.Kind |= docker.EnvValueTypeNumber
		break
	case "select":
		if field["multiple"] == "true" {
			valueRule.Kind |= docker.EnvValueTypeSelectMultiple
		} else {
			valueRule.Kind |= docker.EnvValueTypeSelect
		}
	}

	if call != nil {
		call(valueRule)
	}
	return valueRule
}

func (self Store) parseOnePanelSetting(getter *function.YamlGetter, root string) []docker.EnvItem {
	result := make([]docker.EnvItem, 0)
	fields := getter.GetSliceStringMapString(root)

	for index, field := range fields {
		labels := function.PluckMapWalk(getter.GetStringMapString(fmt.Sprintf("%s.%d.label", root, index)), func(k string, v string) bool {
			return function.InArray([]string{
				"zh", "en",
			}, k)
		})
		if len(labels) == 0 {
			labels = map[string]string{
				"zh": field["labelZh"],
				"en": field["labelEn"],
			}
		}

		envItem := docker.EnvItem{
			Label:  field["labelZh"],
			Labels: labels,
			Name:   field["envKey"],
			Value:  field["default"],
			Rule: &docker.ValueRuleItem{
				Kind:   0,
				Option: make([]docker.ValueItem, 0),
			},
		}
		envItem.Rule = self.ParseSettingField(field, func(item *docker.ValueRuleItem) {
			if (item.Kind&docker.EnvValueTypeSelect) != 0 || (item.Kind&docker.EnvValueTypeSelectMultiple) != 0 {
				item.Option = function.PluckArrayWalk(
					getter.GetSliceStringMapString(fmt.Sprintf("%s.%d.values", root, index)),
					func(i map[string]string) (docker.ValueItem, bool) {
						return docker.ValueItem{
							Name:  i["label"],
							Value: i["value"],
						}, true
					},
				)
			}
		})
		result = append(result, envItem)
	}
	return result
}
