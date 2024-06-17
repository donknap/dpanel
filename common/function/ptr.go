package function

import "time"

func PtrTime(v time.Time) *time.Time {
	return &v
}

func PtrString(str string) *string {
	return &str
}

func PtrBool(b bool) *bool {
	return &b
}
