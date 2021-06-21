package util

import (
	"math/rand"
	"time"
)

func newRandSource() *rand.Rand {
	s1 := rand.NewSource(time.Now().UnixNano())
	return rand.New(s1)
}

func GetRandomIntInRange(n int) int {
	return newRandSource().Intn(n)
}

func GetRandDirName(n int) string {
	runes := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = runes[newRandSource().Intn(len(runes))]
	}
	return string(b)
}
