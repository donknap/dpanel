package function

func Ptr[T any](b T) *T {
	return &b
}
