package logic

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/storage"
	"gopkg.in/yaml.v3"
)

func (self Store) GetAppByBaoTa(storePath string) ([]accessor.StoreAppItem, error) {
	if !filepath.IsAbs(storePath) {
		storePath = filepath.Join(storage.Local{}.GetStorePath(), storePath)
	}
	content, err := os.ReadFile(filepath.Join(storePath, "apps.json"))
	if err != nil {
		return nil, err
	}
	var jsonData []interface{}
	err = json.Unmarshal(content, &jsonData)
	if err != nil {
		return nil, err
	}
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
			Name:  config.GetString("appname"),
			Title: config.GetString("apptitle"),
		}
		result = append(result, storeItem)
	}
	return result, nil
}
