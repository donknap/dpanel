package function

import (
	"cmp"
)

func IsEmptyArray[T interface{}](v []T) bool {
	if v == nil {
		return true
	}
	if len(v) == 0 {
		return true
	}
	return false
}

func InArray[T cmp.Ordered](v []T, item T) bool {
	if v == nil {
		return false
	}
	for _, t := range v {
		if t == item {
			return true
		}
	}
	return false
}

func GetArrayFromMapKeys[T cmp.Ordered](v map[T]interface{}) []T {
	keys := make([]T, 0)
	for key, _ := range v {
		keys = append(keys, key)
	}
	return keys
}

func ConvertArray[T any](interfaces []interface{}) []T {
	slice := make([]T, len(interfaces))
	for i, v := range interfaces {
		if val, ok := v.(T); ok {
			slice[i] = val
		}
	}
	return slice
}
