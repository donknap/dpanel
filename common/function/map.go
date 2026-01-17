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

func PluckMapItemWalk[K cmp.Ordered, U interface{}](v map[K]U, walk func(k K, v U) bool) (U, K, bool) {
	var item U
	var key K
	for key, item = range v {
		if ok := walk(key, item); ok {
			return item, key, true
		}
	}
	return item, key, false
}

func PluckMapWithKeys[K cmp.Ordered, U interface{}](v map[K]U, keys []K) map[K]U {
	if v == nil {
		return map[K]U{}
	}
	return PluckMapWalk(v, func(key K, value U) bool {
		return InArray(keys, key)
	})
}
