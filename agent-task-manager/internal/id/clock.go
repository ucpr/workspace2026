package id

import "time"

func defaultNowMilli() int64 {
	return time.Now().UnixMilli()
}
