package function

import (
	"cmp"
	"sort"
)

func IsEmptyMap[K cmp.Ordered, V interface{}](v map[K]V) bool {
	if v == nil {
		return true
	}
	if len(v) == 0 {
		return true
	}
	return false
}

func PluckMapWalkArray[K cmp.Ordered, U interface{}, R interface{}](m map[K]U, walk func(k K, v U) (R, bool)) []R {
	var keys []K
	for key := range m {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})
	result := make([]R, 0)
	for _, key := range keys {
		newItem, ok := walk(key, m[key])
		if ok {
			result = append(result, newItem)
		}
	}
	return result
}

func PluckMapWalk[K cmp.Ordered, U interface{}](m map[K]U, walk func(k K, v U) bool) map[K]U {
	var keys []K
	for key := range m {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})
	result := make(map[K]U)
	for _, key := range keys {
		if walk(key, m[key]) {
			result[key] = m[key]
		}
	}
	return result
}
