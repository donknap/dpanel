package function

import "time"

func PtrTime(v time.Time) *time.Time {
	return &v
}

func PtrString(str string) *string {
	return &str
}
