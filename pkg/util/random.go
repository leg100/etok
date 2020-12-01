package util

import (
	"math/rand"
	"time"
)

var alphanumeric = "abcdefghijkmnopqrstuvwxyz0123456789"

func GenerateRandomString(size int) string {
	// Without this, Go would generate the same random sequence each run.
	rand.Seed(time.Now().UnixNano())

	buf := make([]byte, size)
	for i := 0; i < size; i++ {
		buf[i] = alphanumeric[rand.Intn(len(alphanumeric))]
	}
	return string(buf)
}
