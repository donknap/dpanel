package logic

import (
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/storage"
	"gopkg.in/yaml.v3"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type CronTemplateItem struct {
	Name        string             `json:"name"`
	Environment []accessor.EnvItem `json:"environment"`
	Script      string             `json:"script"`
	Description string             `json:"description"`
	Tag         []string           `json:"tag"`
}

type CronTemplate struct {
}

func (self CronTemplate) Template() ([]CronTemplateItem, error) {
	result := make([]CronTemplateItem, 0)
	err := filepath.Walk(storage.Local{}.GetScriptTemplatePath(), func(path string, info fs.FileInfo, err error) error {
		if strings.HasSuffix(path, "data.yaml") {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			yamlData := new(function.YamlGetter)
			err = yaml.Unmarshal(content, &yamlData)
			if err != nil {
				return err
			}
			item := CronTemplateItem{
				Name:        yamlData.GetString("task.name"),
				Script:      yamlData.GetString("task.script"),
				Environment: make([]accessor.EnvItem, 0),
				Description: yamlData.GetString("task.descriptionZh"),
				Tag:         yamlData.GetStringSlice("task.tag"),
			}
			fields := yamlData.GetSliceStringMapString("task.environment")
			for _, field := range fields {
				item.Environment = append(item.Environment, accessor.EnvItem{
					Name:  field["name"],
					Label: field["labelZh"],
				})
			}
			result = append(result, item)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
