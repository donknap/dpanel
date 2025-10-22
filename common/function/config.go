package function

import (
	"fmt"
	"strings"

	"github.com/spf13/cast"
)

type ConfigMap map[string]interface{}

func (self ConfigMap) GetString(path string) string {
	return cast.ToString(self.getValueInterface(path))
}

// GetStringSlice
// 获取一个数组类型的 yaml 字段 tag
// tag:
//   - a
//   - b
func (self ConfigMap) GetStringSlice(path string) []string {
	return cast.ToStringSlice(self.toSlice(self.getValueInterface(path)))
}

// GetSliceStringMapString
// 获取一个键值对数组 tag or tag.2.values
// tag:
//   - name: a
//     age: 1
//   - name: b
//     age: 2
//   - name: c
//     age: 3
//     values:
//   - a
//   - b
func (self ConfigMap) GetSliceStringMapString(path string) []map[string]string {
	result := make([]map[string]string, 0)
	slice := cast.ToSlice(self.toSlice(self.getValueInterface(path)))
	for _, item := range slice {
		temp := make(map[string]string)
		for key, value := range item.(ConfigMap) {
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
func (self ConfigMap) GetStringMapString(path string) map[string]string {
	result := make(map[string]string)
	v := self.getValueInterface(path)
	if v == nil {
		return make(map[string]string)
	}
	for key, value := range self.getValueInterface(path).(ConfigMap) {
		result[key] = cast.ToString(value)
	}
	return result
}

func (self ConfigMap) getValueInterface(path string) interface{} {
	if self == nil {
		return interface{}(nil)
	}

	current := self
	pathList := strings.Split(path, ".")
	pathLen := len(pathList)

	for i := 0; i < pathLen; i++ {
		switch t := current[pathList[i]].(type) {
		case []interface{}:
			// 断言是数组类型时，需要转换成 map 再继续下一步
			temp := make(ConfigMap)
			for j, v := range t {
				temp[fmt.Sprintf("%d", j)] = v
			}
			current = temp
		case ConfigMap:
			current = t
		default:
			// 类型非 map 或是 数组，直接返回数据上层再进行转换
			return t
		}
		if i == pathLen-1 {
			return current
		}
	}
	return interface{}(nil)
}

func (self ConfigMap) toSlice(data interface{}) []interface{} {
	if temp, ok := data.(ConfigMap); ok {
		result := make([]interface{}, len(temp))
		for key, value := range temp {
			k := cast.ToInt(key)
			result[k] = value
		}
		return result
	} else {
		return make([]interface{}, 0)
	}
}
