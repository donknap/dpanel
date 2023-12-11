package ptr

import "time"

func Time(v time.Time) *time.Time {
	return &v
}
