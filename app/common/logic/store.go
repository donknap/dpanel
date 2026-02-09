package logic

import (
	"archive/zip"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	exec2 "os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/exec/local"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"gopkg.in/yaml.v3"
)

type StoreLogoFileSystem struct {
	fs.FS
}

func (self StoreLogoFileSystem) Open(name string) (fs.File, error) {
	return os.Open(filepath.Join(storage.Local{}.GetStorePath(), name))
}

type Store struct {
}

type SyncByGitOption struct {
	TargetPath       string
	TempDownloadPath string
}

func (self Store) SyncByGit(gitUrl string, option SyncByGitOption) error {
	if _, err := exec2.LookPath("git"); err != nil {
		return function.ErrorMessage(define.ErrorMessageSystemStoreNotFoundGit)
	}
	var branch string

	if b, a, ok := strings.Cut(gitUrl, "#"); ok {
		gitUrl = b
		branch = a
	}

	// 先创建一个临时目录，下载完成后再同步数据，否则失败时原先的数据会被删除
	if option.TempDownloadPath == "" {
		// 仅当内部生成临时目录的时候才删除，如果是外部传递的，由外部来维护
		option.TempDownloadPath, _ = storage.Local{}.CreateTempDir("")
		defer func() {
			_ = os.RemoveAll(option.TempDownloadPath)
		}()
	}

	slog.Debug("store git download", "path", option.TempDownloadPath)

	args := []string{
		"clone", "--depth", "1",
	}
	if branch != "" {
		args = append(args, "-b", branch)
	}
	args = append(args, gitUrl, option.TempDownloadPath)
	cmd, err := local.New(
		local.WithCommandName("git"),
		local.WithArgs(args...),
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
	if option.TargetPath != "" {
		err = os.RemoveAll(option.TargetPath)
		if err != nil {
			return err
		}
		err = os.CopyFS(option.TargetPath, os.DirFS(option.TempDownloadPath))
		if err != nil {
			return err
		}
	}
	return nil
}

// SyncByZip 同步远程 zip
// root 只同步 root 目录下的内容
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

func (self Store) SyncByUrl(targetPath, url string) error {
	_ = os.MkdirAll(filepath.Dir(targetPath), os.ModePerm)
	file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0666)
	defer func() {
		_ = file.Close()
	}()

	response, err := http.Get(url)
	if err != nil {
		return err
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusOK {
		return function.ErrorMessage(define.ErrorMessageSystemStoreDownloadFailed, "url", url, "error", response.Status)
	}
	_, err = io.Copy(file, response.Body)
	if err != nil {
		return err
	}
	return nil
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
		yamlData := new(function.ConfigMap)
		err = yaml.Unmarshal(composeYaml, &yamlData)
		if err != nil {
			return err
		}
		storeItem.Description = yamlData.GetString("x-casaos.description.zh_cn")
		storeItem.Descriptions = map[string]string{
			define.LangZh: yamlData.GetString("x-casaos.description.zh_cn"),
			define.LangEn: yamlData.GetString("x-casaos.description.en_us"),
		}
		storeItem.Tag = []string{
			yamlData.GetString("x-casaos.category"),
		}
		storeItem.Logo = yamlData.GetString("x-casaos.icon")
		if v := yamlData.GetString("x-casaos.tips.before_install.zh_cn"); v != "" {
			storeItem.Content = "markdown-file://" + v
			storeItem.Contents[define.LangZh] = "markdown://" + v
		}
		if v := yamlData.GetString("x-casaos.tips.before_install.en_us"); v != "" {
			storeItem.Contents[define.LangEn] = "markdown://" + v
		}
		resourcePath, _ := filepath.Rel(filepath.Dir(filepath.Dir(storePath)), appPath)
		storeItem.Version["latest"] = accessor.StoreAppVersionItem{
			Name:        "latest",
			ComposeFile: filepath.Join(resourcePath, "docker-compose.yml"),
			Environment: make([]types.EnvItem, 0),
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

func (self Store) ParseSettingField(field map[string]string, call func(item *types.EnvValueRule)) *types.EnvValueRule {
	valueRule := &types.EnvValueRule{}

	if field["required"] == "true" {
		valueRule.Kind |= types.EnvValueRuleRequired
	}
	if field["disabled"] == "true" {
		valueRule.Kind |= types.EnvValueRuleDisabled
	}

	switch field["type"] {
	case "text":
		valueRule.Kind |= types.EnvValueTypeText
		break
	case "number":
		valueRule.Kind |= types.EnvValueTypeNumber
		break
	case "select":
		if field["multiple"] == "true" {
			valueRule.Kind |= types.EnvValueTypeSelectMultiple
		} else {
			valueRule.Kind |= types.EnvValueTypeSelect
		}
	}

	if call != nil {
		call(valueRule)
	}
	return valueRule
}
