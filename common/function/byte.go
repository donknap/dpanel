package function

// 根据回调函数清除bytes中的字符

func BytesCleanFunc[T byte | rune](data []T, callback func(b T) bool) []T {
	var result []T
	for _, b := range data {
		if !callback(b) {
			result = append(result, b)
		}
	}
	return result
}
