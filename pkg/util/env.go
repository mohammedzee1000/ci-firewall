package util

import (
	"log"
	"os"
)

// Simple helper function to read an environment or return a default value
func getEnv(key string, defaultVal string, failifnotfound bool) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	if failifnotfound {
		log.Fatalf("required env %s not set", key)
	}
	return defaultVal
}
