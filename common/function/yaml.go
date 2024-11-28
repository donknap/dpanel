package function

import (
	"github.com/spf13/cast"
	"strings"
)

type YamlGetter map[string]interface{}

func (self YamlGetter) GetString(path string) string {
	return cast.ToString(self.getValueInterface(path))
}

func (self YamlGetter) GetStringSlice(path string) []string {
	return cast.ToStringSlice(self.getValueInterface(path))
}

func (self YamlGetter) GetSliceStringMapString(path string) []map[string]string {
	result := make([]map[string]string, 0)
	slice := cast.ToSlice(self.getValueInterface(path))
	for _, item := range slice {
		temp := make(map[string]string)
		for key, value := range item.(YamlGetter) {
			temp[key] = cast.ToString(value)
		}
		result = append(result, temp)
	}
	return result
}

func (self YamlGetter) getValueInterface(path string) interface{} {
	if self == nil {
		return interface{}(nil)
	}

	current := self
	pathList := strings.Split(path, ".")
	pathLen := len(pathList)

	for i := 0; i < pathLen; i++ {
		if i == pathLen-1 {
			return current[pathList[i]]
		}
		if _, ok := current[pathList[i]].(YamlGetter); ok {
			current = current[pathList[i]].(YamlGetter)
		}
	}
	return interface{}(nil)
}
