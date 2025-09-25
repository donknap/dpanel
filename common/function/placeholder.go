package function

type Replacer[T any] func(v *T)

func NewReplacerTable[T interface{}](call ...Replacer[T]) []Replacer[T] {
	return call
}

func Placeholder[T any](value *T, replaceFunc ...Replacer[T]) {
	for _, replacer := range replaceFunc {
		if replacer != nil {
			replacer(value)
		}
	}
}
