package function

import (
	"cmp"
	"reflect"
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

func InArrayArray[T cmp.Ordered](v []T, item ...T) bool {
	if v == nil {
		return false
	}
	for _, t := range item {
		if found := InArray(v, t); found {
			return true
		}
	}
	return false
}

func InArrayWalk[T interface{}](v []T, walk func(i T) bool) bool {
	exists, _ := IndexArrayWalk(v, walk)
	return exists
}

func IndexArrayWalk[T interface{}](v []T, walk func(i T) bool) (exists bool, index int) {
	if v == nil {
		return false, 0
	}
	for i, t := range v {
		if walk(t) {
			return true, i
		}
	}
	return false, 0
}

func PluckArrayWalk[T interface{}, R interface{}](v []T, walk func(i T) (R, bool)) []R {
	result := make([]R, 0)
	for _, item := range v {
		newItem, ok := walk(item)
		if ok {
			result = append(result, newItem)
		}
	}
	return result
}

func FindArrayValueIndex(items interface{}, value ...interface{}) (exists bool, pos []int) {
	pos = make([]int, 0)
	some := reflect.ValueOf(items)

	switch some.Kind() {
	case reflect.Slice, reflect.Array:
		for i := 0; i < some.Len(); i++ {
			if len(value) == 1 {
				if reflect.DeepEqual(some.Index(i).Interface(), value[0]) {
					pos = append(pos, i)
				}
			} else {
				someStruct := some.Index(i)
				if someStruct.Kind() == reflect.Ptr {
					someStruct = someStruct.Elem()
				}
				if someStruct.Kind() == reflect.Struct {
					fieldName, ok := value[0].(string)
					someValue := someStruct.FieldByName(fieldName)
					if ok {
						if someValue.Kind() == reflect.Slice || someValue.Kind() == reflect.Array {
							exists1, _ := FindArrayValueIndex(someValue.Interface(), value[1:]...)
							if exists1 {
								return exists1, append(pos, i)
							}
						} else {
							for j := 1; j < len(value)-1; j++ {
								fieldName, ok = value[j].(string)
								someValue = someValue.FieldByName(fieldName)
							}
							if reflect.DeepEqual(someValue.Interface(), value[len(value)-1]) {
								pos = append(pos, i)
							}
						}
					} else {
						return false, pos
					}
				} else {
					return false, pos
				}
			}
		}
	default:
		return false, pos
	}
	if len(pos) > 0 {
		return true, pos
	} else {
		return false, nil
	}
}
