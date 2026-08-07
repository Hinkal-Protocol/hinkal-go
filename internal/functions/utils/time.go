package utils

import "time"

func GetCurrentTimeInSeconds() int64 {
	return time.Now().Unix()
}
