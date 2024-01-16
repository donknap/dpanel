package function

import "cmp"

func IsEmptyMap[K cmp.Ordered, V interface{}](v map[K]V) bool {
	if v == nil {
		return true
	}
	if len(v) == 0 {
		return true
	}
	return false
}
