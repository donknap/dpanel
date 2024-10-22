package function

import (
	"cmp"
	"fmt"
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

func ConvertArray[T any](interfaces []interface{}) []T {
	slice := make([]T, len(interfaces))
	for i, v := range interfaces {
		if val, ok := v.(T); ok {
			slice[i] = val
		}
	}
	return slice
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
				if some.Index(i).Kind() == reflect.Struct {
					fieldName, ok := value[0].(string)
					someValue := some.Index(i).FieldByName(fieldName)
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
					fmt.Printf("%v \n", 1111)
					fieldName, ok := value[0].(string)
					if ok {
						someValue := some.Index(i).FieldByName(fieldName)
						return FindArrayValueIndex(someValue, value[1:]...)
					} else {
						return false, pos
					}
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
	//for i, item := range items {
	//	itemValue := reflect.ValueOf(item)
	//	fieldValue := itemValue.FieldByName(fieldName)
	//	if !fieldValue.IsValid() {
	//		fmt.Printf("Unknown field name: %s\n", fieldName)
	//		return -1
	//	}
	//	if reflect.DeepEqual(fieldValue.Interface(), value) {
	//		return i
	//	}
	//}
	//return -1
}
