package function

import "time"

func PtrTime(v time.Time) *time.Time {
	return &v
}
