package tools

import "time"

// Millisecond - 获取当前时间毫秒数
func Millisecond() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
