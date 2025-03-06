package function

import (
	"fmt"
	"github.com/spf13/cast"
	"strings"
)

type YamlGetter map[string]interface{}

func (self YamlGetter) GetString(path string) string {
	return cast.ToString(self.getValueInterface(path))
}

// GetStringSlice
// 获取一个数组类型的 yaml 字段
// tag:
//   - a
//   - b
func (self YamlGetter) GetStringSlice(path string) []string {
	return cast.ToStringSlice(self.getValueInterface(path))
}

// GetSliceStringMapString
// 获取一个键值对数组
// tag:
//   - name: a
//     age: 1
//   - name: b
//     age: 2
func (self YamlGetter) GetSliceStringMapString(path string) []map[string]string {
	result := make([]map[string]string, 0)
	temp := make([]interface{}, 0)
	for _, value := range self.getValueInterface(path).(YamlGetter) {
		temp = append(temp, value)
	}
	slice := cast.ToSlice(temp)
	for _, item := range slice {
		temp := make(map[string]string)
		for key, value := range item.(YamlGetter) {
			temp[key] = cast.ToString(value)
		}
		result = append(result, temp)
	}
	return result
}

// GetStringMapString
// 获取一个键值对
// tag:
//
//	name: a
//	age: 1
func (self YamlGetter) GetStringMapString(path string) map[string]string {
	result := make(map[string]string)
	for key, value := range self.getValueInterface(path).(YamlGetter) {
		result[key] = cast.ToString(value)
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
		switch t := current[pathList[i]].(type) {
		case []interface{}:
			temp := make(YamlGetter)
			for j, v := range t {
				temp[fmt.Sprintf("%d", j)] = v.(YamlGetter)
			}
			current = temp
		case YamlGetter:
			current = current[pathList[i]].(YamlGetter)
		}
		if i == pathLen-1 {
			return current
		}
	}
	return interface{}(nil)
}
