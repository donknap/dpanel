package logic

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"gopkg.in/yaml.v3"
)

func (self Store) GetAppByBaoTa(storePath string, downloadUrl string) ([]accessor.StoreAppItem, error) {
	if !filepath.IsAbs(storePath) {
		storePath = filepath.Join(storage.Local{}.GetStorePath(), storePath)
	}
	_ = os.RemoveAll(storePath)
	urls, err := url.Parse(downloadUrl)
	if err != nil {
		return nil, err
	}
	fileName := "apps.json"
	err = self.SyncByUrl(filepath.Join(storePath, fileName), urls.JoinPath(fileName).String())
	if err != nil {
		return nil, err
	}
	fileName = "dkapp_ico.zip"
	err = self.SyncByUrl(filepath.Join(storePath, fileName), urls.JoinPath(fileName).String())
	if err != nil {
		return nil, err
	}
	appsJson, err := os.ReadFile(filepath.Join(storePath, "apps.json"))
	if err != nil {
		return nil, err
	}
	logoZip, err := zip.OpenReader(filepath.Join(storePath, "dkapp_ico.zip"))
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = logoZip.Close()
	}()

	var jsonData []interface{}
	err = json.Unmarshal(appsJson, &jsonData)
	if err != nil {
		return nil, err
	}

	r := time.Now().Unix()

	result := make([]accessor.StoreAppItem, 0)
	for _, item := range jsonData {
		itemData, err := yaml.Marshal(item)
		if err != nil {
			continue
		}
		config := new(function.ConfigMap)
		err = yaml.Unmarshal(itemData, &config)
		if err != nil {
			continue
		}
		storeItem := accessor.StoreAppItem{
			Title:    config.GetString("apptitle"),
			Name:     config.GetString("appname"),
			Logo:     "",
			Contents: map[string]string{},
			Descriptions: map[string]string{
				define.LangZh: config.GetString("appdesc"),
			},
			Tag: []string{
				config.GetString("apptype"), config.GetString("appTypeCN"),
			},
			Website: config.GetString("home"),
			Version: make(map[string]accessor.StoreAppVersionItem),
		}
		storeItemPath := filepath.Join(storePath, "apps", storeItem.Name)
		resourceStoreItemPath, _ := filepath.Rel(storage.Local{}.GetStorePath(), storeItemPath)

		_ = os.MkdirAll(storeItemPath, os.ModePerm)

		if logo, _, ok := function.PluckArrayItemWalk(logoZip.File, func(item *zip.File) bool {
			return fmt.Sprintf("dkapp_ico/ico-dkapp_%s.png", storeItem.Name) == item.Name
		}); ok {
			if reader, err := logo.Open(); err == nil {
				if f, err := os.OpenFile(filepath.Join(storeItemPath, filepath.Base(logo.Name)), os.O_CREATE|os.O_TRUNC|os.O_RDWR, logo.Mode()); err == nil {
					_, err := io.Copy(f, reader)
					_ = f.Close()
					if err != nil {
						slog.Debug("store bt sync copy icon", "err", err)
					} else {
						storeItem.Logo = fmt.Sprintf("image://%s/%s?r=%d", strings.ReplaceAll(resourceStoreItemPath, string(filepath.Separator), "/"), filepath.Base(logo.Name), r)
					}
				}
			}
		}

		environment := make([]types.EnvItem, 0)
		if envList := config.GetSliceStringMapString("env"); !function.IsEmptyArray(envList) {
			for _, field := range envList {
				envItem := types.EnvItem{
					Label: "",
					Labels: map[string]string{
						define.LangZh: field["desc"],
					},
					Name:  strings.ToUpper(field["key"]),
					Value: field["default"],
				}
				envItem.Rule = self.ParseSettingField(field, nil)
				environment = append(environment, envItem)
			}
		}

		if version := config.GetSliceStringMapString("appversion"); !function.IsEmptyArray(version) && len(version) > 0 {
			firstVersionName := ""
			for i, versionItem := range version {
				mVersion := versionItem["m_version"]
				sVersion := config.GetStringSlice(fmt.Sprintf("appversion.%d.s_version", i))
				if function.IsEmptyArray(sVersion) {
					storeItem.Version[mVersion] = accessor.StoreAppVersionItem{
						Name:        mVersion,
						ComposeFile: filepath.Join(resourceStoreItemPath, "docker-compose.yml"),
						Environment: environment,
						Script:      make(map[string]string),
						Download:    urls.JoinPath("templates", storeItem.Name+".zip").String(),
						Default:     true,
					}
					firstVersionName = mVersion
				} else {
					for _, s := range sVersion {
						versionName := fmt.Sprintf("%s.%s", mVersion, s)
						if firstVersionName == "" {
							storeItem.Version[versionName] = accessor.StoreAppVersionItem{
								Name:        versionName,
								ComposeFile: filepath.Join(resourceStoreItemPath, "docker-compose.yml"),
								Environment: environment,
								Script:      make(map[string]string),
								Download:    urls.JoinPath("templates", storeItem.Name+".zip").String(),
								Default:     true,
							}
							firstVersionName = versionName
						} else {
							storeItem.Version[versionName] = accessor.StoreAppVersionItem{
								Name: versionName,
								Ref:  firstVersionName,
							}
						}
					}
				}
			}
		}

		//go func() {
		//	wg.Add(1)
		//	defer func() {
		//		wg.Done()
		//	}()
		//	saveZipPath := filepath.Join(storePath, "templates", storeItem.Name+".zip")
		//	err = self.SyncByUrl(saveZipPath, urls.JoinPath("templates", filepath.Base(saveZipPath)).String())
		//	if err != nil {
		//		slog.Debug("store bt sync download zip", "name", storeItem.Name, "err", err)
		//		return
		//	}
		//	err := function.Unzip(filepath.Join(storePath, "apps"), saveZipPath)
		//	if err != nil {
		//		slog.Debug("store bt sync unzip zip", "name", storeItem.Name, "err", err)
		//		return
		//	}
		//}()
		result = append(result, storeItem)
	}
	_ = os.RemoveAll(filepath.Join(storePath, "templates"))
	return result, nil
}
