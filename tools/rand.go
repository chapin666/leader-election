package tools

import "math/rand"

// RandRange return a int64
func RandRange(min, max int64) int64 {
	return rand.Int63n(max-min) + min
}
