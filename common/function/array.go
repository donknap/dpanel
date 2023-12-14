package function

import "cmp"

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
